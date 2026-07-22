// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"dogego/chain"
	"dogego/store"
)

// execImportMultiWallet batch-imports watch-only scripts for the built-in wallet.
func execImportMultiWallet(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if strings.TrimSpace(string(params[0])) == "null" {
		return nil, -8, "importmulti: requests must be a JSON array"
	}
	var reqs []json.RawMessage
	if err := json.Unmarshal(params[0], &reqs); err != nil {
		return nil, -8, "importmulti: requests must be a JSON array"
	}
	if len(reqs) == 0 {
		return nil, -8, "importmulti: requests must not be empty"
	}
	rescan := true
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var opts map[string]json.RawMessage
		if err := json.Unmarshal(params[1], &opts); err != nil {
			return nil, -8, "importmulti: options must be a JSON object"
		}
		if rawRescan, ok := opts["rescan"]; ok && strings.TrimSpace(string(rawRescan)) != "null" {
			var c int
			var m string
			rescan, c, m = parseRPCBoolOpt(rawRescan, true, "importmulti", "rescan")
			if c != 0 {
				return nil, c, m
			}
		}
	}
	if paths == nil || paths.WalletImportWatch == nil || rpcWalletDefaultAddress(paths) == "" {
		return nil, -1, "importmulti: wallet is not implemented in DogeGo"
	}
	out := make([]map[string]interface{}, 0, len(reqs))
	anyOK := false
	for _, elem := range reqs {
		row := importMultiOne(chainName, paths, elem)
		out = append(out, row)
		if ok, _ := row["success"].(bool); ok {
			anyOK = true
		}
	}
	if rescan && anyOK {
		if code, msg := walletRescanAfterImport(paths, j, raw, nil, -1, "importmulti"); code != 0 {
			return nil, code, msg
		}
	}
	return out, 0, ""
}

func importMultiOne(chainName string, paths *DataPaths, elem json.RawMessage) map[string]interface{} {
	fail := func(code int, message string) map[string]interface{} {
		return map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    code,
				"message": message,
			},
		}
	}
	var req map[string]json.RawMessage
	if err := json.Unmarshal(elem, &req); err != nil {
		return fail(-8, "importmulti: each request must be a JSON object")
	}
	if rawDesc, ok := req["desc"]; ok && strings.TrimSpace(string(rawDesc)) != "null" {
		return importMultiFromDesc(chainName, paths, req)
	}
	if rawKeys, ok := req["keys"]; ok && strings.TrimSpace(string(rawKeys)) != "null" {
		var keyElems []json.RawMessage
		if err := json.Unmarshal(rawKeys, &keyElems); err == nil && len(keyElems) > 0 {
			return importMultiFromKeys(chainName, paths, req)
		}
	}
	if rawPubkeys, ok := req["pubkeys"]; ok && strings.TrimSpace(string(rawPubkeys)) != "null" {
		return importMultiFromPubkeys(chainName, paths, req)
	}
	spkRaw, ok := req["scriptPubKey"]
	if !ok || strings.TrimSpace(string(spkRaw)) == "null" {
		return fail(-8, "importmulti: missing scriptPubKey or desc")
	}
	if rawRedeem, ok := req["redeemscript"]; ok && strings.TrimSpace(string(rawRedeem)) != "null" {
		var redeem string
		if err := json.Unmarshal(rawRedeem, &redeem); err != nil {
			return fail(-8, "importmulti: redeemscript must be a hex string")
		}
		redeem = strings.TrimSpace(redeem)
		if redeem == "" {
			return fail(-8, "importmulti: redeemscript must be a hex string")
		}
		pk, err := hex.DecodeString(strings.TrimPrefix(strings.ToLower(redeem), "0x"))
		if err != nil || len(pk) == 0 {
			return fail(-8, "importmulti: redeemscript must be a hex string")
		}
		h := chain.Hash160(pk)
		if len(h) != 20 {
			return fail(-8, "importmulti: redeemscript must be a hex string")
		}
		var h160 [20]byte
		copy(h160[:], h)
		p2sh := chain.P2SHScriptFromScriptHash(h160)
		if err := rpcWalletImportWatchScript(paths, p2sh, pk); err != nil {
			return fail(-1, "importmulti: "+err.Error())
		}
		walletApplyLabel(chainName, paths, p2sh, walletLabelFromImportMultiReq(req))
		walletRecordImportMultiRedeem(chainName, paths, p2sh, pk, req)
		return map[string]interface{}{"success": true}
	}
	arg, code, msg := importMultiScriptPubKeyArg(spkRaw)
	if code != 0 {
		return fail(code, msg)
	}
	pkScript, code, msg := importWatchScriptArg(chainName, arg, false)
	if code != 0 {
		return fail(code, msg)
	}
	if err := paths.WalletImportWatch(pkScript); err != nil {
		return fail(-1, "importmulti: "+err.Error())
	}
	walletApplyLabel(chainName, paths, pkScript, walletLabelFromImportMultiReq(req))
	walletRecordImportMulti(chainName, paths, pkScript, req, false)
	return map[string]interface{}{"success": true}
}

func importMultiFromDesc(chainName string, paths *DataPaths, req map[string]json.RawMessage) map[string]interface{} {
	fail := func(code int, message string) map[string]interface{} {
		return map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    code,
				"message": message,
			},
		}
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return fail(-8, "importmulti: "+err.Error())
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return fail(-8, "importmulti: "+err.Error())
	}
	elem, err := json.Marshal(req)
	if err != nil {
		return fail(-1, "importmulti: internal error")
	}
	row, ok := importDescriptorOne(chainName, paths, p, elem)
	if !ok {
		if errMap, ok2 := row["error"].(map[string]interface{}); ok2 {
			code := -8
			if c, ok3 := errMap["code"].(float64); ok3 {
				code = int(c)
			}
			msg, _ := errMap["message"].(string)
			msg = strings.Replace(msg, "importdescriptors:", "importmulti:", 1)
			return fail(code, msg)
		}
		return fail(-1, "importmulti: import failed")
	}
	return map[string]interface{}{"success": true}
}

func importMultiScriptPubKeyArg(raw json.RawMessage) (arg string, code int, msg string) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s), 0, ""
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", -8, "importmulti: scriptPubKey must be a string or object"
	}
	if rawAddr, ok := obj["address"]; ok && strings.TrimSpace(string(rawAddr)) != "null" {
		if err := json.Unmarshal(rawAddr, &s); err != nil {
			return "", -8, "importmulti: scriptPubKey address must be a string"
		}
		return strings.TrimSpace(s), 0, ""
	}
	if rawHex, ok := obj["hex"]; ok && strings.TrimSpace(string(rawHex)) != "null" {
		if err := json.Unmarshal(rawHex, &s); err != nil {
			return "", -8, "importmulti: scriptPubKey hex must be a string"
		}
		return strings.TrimSpace(s), 0, ""
	}
	return "", -8, "importmulti: scriptPubKey must contain address or hex"
}

func importMultiFromPubkeys(chainName string, paths *DataPaths, req map[string]json.RawMessage) map[string]interface{} {
	fail := func(code int, message string) map[string]interface{} {
		return map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    code,
				"message": message,
			},
		}
	}
	rawPubkeys := req["pubkeys"]
	var pubkeys []string
	if err := json.Unmarshal(rawPubkeys, &pubkeys); err != nil || len(pubkeys) == 0 {
		return fail(-8, "importmulti: pubkeys must be a non-empty array")
	}
	nRequired := len(pubkeys)
	if rawReq, ok := req["required"]; ok && strings.TrimSpace(string(rawReq)) != "null" {
		var n json.Number
		if err := json.Unmarshal(rawReq, &n); err != nil {
			return fail(-8, "importmulti: required must be a number")
		}
		ni, err := n.Int64()
		if err != nil || ni < 1 || ni > 16 {
			return fail(-8, "importmulti: required out of range")
		}
		nRequired = int(ni)
	}
	keysJ, err := json.Marshal(pubkeys)
	if err != nil {
		return fail(-1, "importmulti: internal error")
	}
	nJ, err := json.Marshal(nRequired)
	if err != nil {
		return fail(-1, "importmulti: internal error")
	}
	m, code, msg := execCreateMultisig(chainName, []json.RawMessage{nJ, keysJ})
	if code != 0 {
		return fail(code, msg)
	}
	redeemHex, _ := m["redeemScript"].(string)
	if redeemHex == "" {
		return fail(-1, "importmulti: internal error")
	}
	pk, err := hex.DecodeString(redeemHex)
	if err != nil || len(pk) == 0 {
		return fail(-8, "importmulti: invalid multisig script")
	}
	h := chain.Hash160(pk)
	if len(h) != 20 {
		return fail(-8, "importmulti: invalid multisig script")
	}
	var h160 [20]byte
	copy(h160[:], h)
	p2sh := chain.P2SHScriptFromScriptHash(h160)
	if err := rpcWalletImportWatchScript(paths, p2sh, pk); err != nil {
		return fail(-1, "importmulti: "+err.Error())
	}
	if lbl := walletLabelFromImportMultiReq(req); lbl != "" && paths.WalletSetLabel != nil {
		if msAddr, _ := m["address"].(string); msAddr != "" {
			_ = paths.WalletSetLabel(msAddr, lbl)
		}
	}
	if msAddr, _ := m["address"].(string); msAddr != "" {
		if _, h160, derr := chain.Base58CheckDecode(msAddr); derr == nil {
			p2sh := chain.P2SHScriptFromScriptHash(h160)
			walletRecordImportMultiRedeem(chainName, paths, p2sh, pk, req)
		}
	}
	return map[string]interface{}{"success": true}
}

func importMultiFromKeys(chainName string, paths *DataPaths, req map[string]json.RawMessage) map[string]interface{} {
	fail := func(code int, message string) map[string]interface{} {
		return map[string]interface{}{
			"success": false,
			"error": map[string]interface{}{
				"code":    code,
				"message": message,
			},
		}
	}
	if paths == nil || paths.WalletImportPrivKey == nil {
		return fail(-1, "importmulti: wallet is not implemented in DogeGo")
	}
	if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
		return fail(code, msg)
	}
	rawKeys := req["keys"]
	var keys []string
	if err := json.Unmarshal(rawKeys, &keys); err != nil {
		var raw []json.RawMessage
		if err2 := json.Unmarshal(rawKeys, &raw); err2 != nil {
			return fail(-8, "importmulti: keys must be an array of strings")
		}
		for _, elem := range raw {
			var k string
			if err := json.Unmarshal(elem, &k); err != nil {
				return fail(-8, "importmulti: keys must be an array of strings")
			}
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return fail(-8, "importmulti: keys must not be empty")
	}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if err := paths.WalletImportPrivKey(k); err != nil {
			return fail(-1, "importmulti: "+err.Error())
		}
	}
	if addr, err := addressFromWIF(chainName, keys[0]); err == nil {
		net, nerr := networkFromRPCChainName(chainName)
		if nerr == nil {
			if p, perr := chain.ParamsFor(net); perr == nil {
				if _, h160, derr := chain.Base58CheckDecode(addr); derr == nil {
					pkScript := chain.P2PKHScriptFromPubKeyHash(h160)
					walletRecordImportMulti(chainName, paths, pkScript, req, true)
				}
				_ = p
			}
		}
	}
	return map[string]interface{}{"success": true}
}
