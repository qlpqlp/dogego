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
	"dogego/consensus"
)

func walletDescriptorForScript(chainName string, pkScript []byte) string {
	if len(pkScript) == 0 {
		return ""
	}
	if desc, ok := consensus.MultiDescriptorFromRedeem(pkScript); ok {
		return desc
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return ""
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return ""
	}
	addr := chain.ScriptPubKeyAddress(pkScript, p.PubkeyHashAddrID, p.ScriptHashAddrID)
	if addr == "" {
		return ""
	}
	if len(pkScript) == 25 && pkScript[0] == 0x76 {
		return "pkh(" + addr + ")"
	}
	if len(pkScript) == 23 && pkScript[0] == 0xa9 {
		if innerAddr := p2shInnerPKHAddress(pkScript, p.PubkeyHashAddrID); innerAddr != "" {
			return "sh(pkh(" + innerAddr + "))"
		}
		return "sh(pkh(" + addr + "))"
	}
	return ""
}

func p2shInnerPKHAddress(p2sh []byte, pkhVer byte) string {
	if len(p2sh) != 23 || p2sh[0] != 0xa9 {
		return ""
	}
	// Cannot derive inner without redeem; caller may pass watch_redeems elsewhere.
	return ""
}

func parseImportMultiDescriptorMeta(req map[string]json.RawMessage) (timestamp int64, internal bool) {
	timestamp = time.Now().Unix()
	if rawInt, ok := req["internal"]; ok && strings.TrimSpace(string(rawInt)) != "null" {
		internal, _, _ = parseRPCBoolOpt(rawInt, false, "importmulti", "internal")
	}
	if rawTs, ok := req["timestamp"]; ok && strings.TrimSpace(string(rawTs)) != "null" {
		var tsNum json.Number
		if err := json.Unmarshal(rawTs, &tsNum); err == nil {
			if n, err := tsNum.Int64(); err == nil && n >= 0 {
				timestamp = n
			}
		} else {
			var tsStr string
			if err := json.Unmarshal(rawTs, &tsStr); err == nil && strings.EqualFold(strings.TrimSpace(tsStr), "now") {
				timestamp = time.Now().Unix()
			}
		}
	}
	return timestamp, internal
}

func walletRecordImportMulti(chainName string, paths *DataPaths, pkScript []byte, req map[string]json.RawMessage, spendable bool) {
	if paths == nil || paths.WalletAddImportedDescriptor == nil || len(pkScript) == 0 {
		return
	}
	desc := walletDescriptorForScript(chainName, pkScript)
	if desc == "" {
		return
	}
	ts, internal := parseImportMultiDescriptorMeta(req)
	_ = paths.WalletAddImportedDescriptor(desc, ts, internal, spendable)
}

// walletDescriptorForP2SHRedeem builds sh(pkh) or sh(multi) when redeem script is known.
func walletDescriptorForP2SHRedeem(chainName string, p2shScript, redeem []byte) string {
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return ""
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return ""
	}
	if desc, ok := consensus.P2SHRedeemDescriptor(redeem, p.PubkeyHashAddrID); ok {
		return desc
	}
	if len(redeem) == 25 && redeem[0] == 0x76 {
		if inner := chain.ScriptPubKeyAddress(redeem, p.PubkeyHashAddrID, p.ScriptHashAddrID); inner != "" {
			return "sh(pkh(" + inner + "))"
		}
	}
	_ = p2shScript
	return ""
}

func walletRecordImportMultiRedeem(chainName string, paths *DataPaths, p2sh, redeem []byte, req map[string]json.RawMessage) {
	desc := walletDescriptorForP2SHRedeem(chainName, p2sh, redeem)
	if desc == "" {
		desc = walletDescriptorForScript(chainName, p2sh)
	}
	if desc == "" || paths == nil || paths.WalletAddImportedDescriptor == nil {
		return
	}
	ts, internal := parseImportMultiDescriptorMeta(req)
	_ = paths.WalletAddImportedDescriptor(desc, ts, internal, false)
}
