// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
	"time"

	"dogego/chain"
	"dogego/store"
)

// execImportDescriptors imports watch-only pkh / sh(pkh) descriptors (Core subset).
func execImportDescriptors(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if strings.TrimSpace(string(params[0])) == "null" {
		return nil, -8, "importdescriptors: must pass an array of descriptors"
	}
	var reqs []json.RawMessage
	if err := json.Unmarshal(params[0], &reqs); err != nil {
		return nil, -8, "importdescriptors: must pass an array of descriptors"
	}
	if len(reqs) == 0 {
		return nil, -8, "importdescriptors: must pass an array of descriptors"
	}
	rescan := true
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var opts map[string]json.RawMessage
		if err := json.Unmarshal(params[1], &opts); err != nil {
			return nil, -8, "importdescriptors: options must be a JSON object"
		}
		if rawRescan, ok := opts["rescan"]; ok && strings.TrimSpace(string(rawRescan)) != "null" {
			var c int
			var m string
			rescan, c, m = parseRPCBoolOpt(rawRescan, true, "importdescriptors", "rescan")
			if c != 0 {
				return nil, c, m
			}
		}
	}
	if !WalletActive(paths) {
		return nil, -1, "importdescriptors: built-in wallet not enabled"
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}
	out := make([]interface{}, 0, len(reqs))
	anyOK := false
	for _, elem := range reqs {
		row, ok := importDescriptorOne(chainName, paths, p, elem)
		out = append(out, row)
		if ok {
			anyOK = true
		}
	}
	if rescan && anyOK {
		if code, msg := walletRescanAfterImport(paths, j, raw, nil, -1, "importdescriptors"); code != 0 {
			return nil, code, msg
		}
	}
	return out, 0, ""
}

func importDescriptorOne(chainName string, paths *DataPaths, p chain.Params, elem json.RawMessage) (map[string]interface{}, bool) {
	fail := func(msg string) map[string]interface{} {
		return map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    -8,
				"message": "importdescriptors: " + msg,
			},
		}
	}
	var req map[string]json.RawMessage
	if err := json.Unmarshal(elem, &req); err != nil {
		return fail("each descriptor must be a JSON object"), false
	}
	rawDesc, ok := req["desc"]
	if !ok {
		return fail("missing desc"), false
	}
	var desc string
	if err := json.Unmarshal(rawDesc, &desc); err != nil {
		return fail("desc must be a string"), false
	}
	parsed, ok := parseImportDescriptorAllowed(paths, desc)
	if !ok {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(desc)), "multi(") && !rpcAllowBareMultisig(paths) {
			return fail("bare multisig descriptors require permitbaremultisig"), false
		}
		return fail("unsupported descriptor (addr, pkh, sh(pkh), sh(multi), sh(cltv/csv multi/pkh), multi only)"), false
	}
	var label string
	if rawLbl, ok := req["label"]; ok && strings.TrimSpace(string(rawLbl)) != "null" {
		_ = json.Unmarshal(rawLbl, &label)
		label = strings.TrimSpace(label)
	}
	internal := false
	if rawInt, ok := req["internal"]; ok && strings.TrimSpace(string(rawInt)) != "null" {
		var code int
		var msg string
		internal, code, msg = parseRPCBoolOpt(rawInt, false, "importdescriptors", "internal")
		if code != 0 {
			return fail(msg), false
		}
	}
	timestamp := time.Now().Unix()
	if rawTs, ok := req["timestamp"]; ok && strings.TrimSpace(string(rawTs)) != "null" {
		var tsNum json.Number
		if err := json.Unmarshal(rawTs, &tsNum); err == nil {
			if n, err := tsNum.Int64(); err == nil && n >= 0 {
				timestamp = n
			} else {
				return fail("timestamp must be a non-negative number"), false
			}
		} else {
			var tsStr string
			if err := json.Unmarshal(rawTs, &tsStr); err != nil {
				return fail("timestamp must be a number or \"now\""), false
			}
			tsStr = strings.TrimSpace(strings.ToLower(tsStr))
			if tsStr != "now" {
				return fail("timestamp must be a number or \"now\""), false
			}
		}
	}
	var importKeys []string
	if rawKeys, ok := req["keys"]; ok && strings.TrimSpace(string(rawKeys)) != "null" {
		if err := json.Unmarshal(rawKeys, &importKeys); err != nil {
			return fail("keys must be a JSON array of strings"), false
		}
	}
	spendImport := len(importKeys) > 0
	if spendImport {
		if paths.WalletImportPrivKey == nil {
			return fail("wallet cannot import private keys"), false
		}
		if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
			return fail(msg), false
		}
		if code, msg := validateDescriptorImportKeys(chainName, parsed, importKeys); code != 0 {
			return fail(msg), false
		}
	}
	if parsed.scriptType == "pkh" {
		vis, _, _ := ValidateAddressString(chainName, parsed.addr)
		if v, _ := vis["isvalid"].(bool); !v {
			return fail("invalid address in descriptor"), false
		}
	}
	var pkScript []byte
	var importAddr string
	innerPKHAddr := parsed.addr
	var okAddr bool
	pkScript, importAddr, okAddr = pkScriptAndAddressFromParsedDescriptor(p, parsed)
	if !okAddr {
		switch parsed.scriptType {
		case "pkh":
			return fail("invalid address in descriptor"), false
		case "p2sh-pkh", "p2sh-cltv-pkh", "p2sh-csv-pkh", "p2sh-multi", "p2sh-cltv-multi", "p2sh-csv-multi":
			return fail("invalid multisig or timelock descriptor"), false
		case "bare-multi":
			return fail("invalid multisig descriptor"), false
		default:
			return fail("unsupported descriptor"), false
		}
	}
	if spendImport && descriptorScriptTypeIsP2SHPKHWithKeys(parsed.scriptType) {
		wif := strings.TrimSpace(importKeys[0])
		if err := paths.WalletImportPrivKey(wif); err != nil {
			if code, msg := rpcWalletOpErr(err); code != 0 {
				return fail(msg), false
			}
			return fail(err.Error()), false
		}
		if parsed.scriptType == "p2sh-pkh" || parsed.scriptType == "p2sh-cltv-pkh" || parsed.scriptType == "p2sh-csv-pkh" {
			if paths.WalletImportWatch == nil {
				return fail("wallet cannot import watch scripts"), false
			}
			if descriptorScriptTypeUsesStoredRedeem(parsed.scriptType) {
				if err := rpcWalletImportWatchScript(paths, pkScript, parsed.redeem); err != nil {
					return fail(err.Error()), false
				}
			} else if err := paths.WalletImportWatch(pkScript); err != nil {
				return fail(err.Error()), false
			}
		}
	} else if spendImport && (descriptorScriptTypeIsP2SHMulti(parsed.scriptType) || parsed.scriptType == "bare-multi") {
		if paths.WalletImportPrivKey == nil {
			return fail("wallet cannot import private keys"), false
		}
		for _, wif := range importKeys {
			wif = strings.TrimSpace(wif)
			if wif == "" {
				continue
			}
			if err := paths.WalletImportPrivKey(wif); err != nil {
				if code, msg := rpcWalletOpErr(err); code != 0 {
					return fail(msg), false
				}
				return fail(err.Error()), false
			}
		}
		if descriptorScriptTypeUsesStoredRedeem(parsed.scriptType) {
			if err := rpcWalletImportWatchScript(paths, pkScript, parsed.redeem); err != nil {
				return fail(err.Error()), false
			}
		} else if err := paths.WalletImportWatch(pkScript); err != nil {
			return fail(err.Error()), false
		}
	} else {
		if paths.WalletImportWatch == nil {
			return fail("wallet cannot import watch scripts"), false
		}
		if descriptorScriptTypeUsesStoredRedeem(parsed.scriptType) {
			if err := rpcWalletImportWatchScript(paths, pkScript, parsed.redeem); err != nil {
				return fail(err.Error()), false
			}
		} else if err := paths.WalletImportWatch(pkScript); err != nil {
			return fail(err.Error()), false
		}
	}
	if label != "" && importAddr != "" && paths.WalletSetLabel != nil {
		_ = paths.WalletSetLabel(importAddr, label)
		if spendImport && parsed.scriptType == "p2sh-pkh" && innerPKHAddr != importAddr {
			_ = paths.WalletSetLabel(innerPKHAddr, label)
		}
	}
	if paths.WalletAddImportedDescriptor != nil {
		_ = paths.WalletAddImportedDescriptor(parsed.normalized, timestamp, internal, spendImport)
	}
	return map[string]interface{}{"success": true, "warnings": []interface{}{}}, true
}
