// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"strings"
)

// enrichWalletAddressMeta adds label, HD path, and pubkey fields (Core getaddressinfo / validateaddress subset).
func enrichWalletAddressMeta(chainName string, paths *DataPaths, addr string, out map[string]interface{}) {
	if paths == nil || out == nil {
		return
	}
	if lbl := rpcWalletGetLabel(paths, addr); lbl != "" {
		out["label"] = lbl
	}
	if paths.WalletAddressHDPath != nil {
		if hdpath, ischg, ok := paths.WalletAddressHDPath(addr); ok {
			out["hdkeypath"] = hdpath
			out["hdmasterkeyid"] = "<hd master key not exposed>"
			if ischg {
				out["ischange"] = true
			}
		}
	}
	if ismine, _ := out["ismine"].(bool); ismine {
		if pub := walletPubKeyHexForAddress(chainName, paths, addr); pub != "" {
			out["pubkey"] = pub
		}
	}
	if rpcWalletAvoidReuse(paths) {
		if spk, ok := out["scriptPubKey"].(map[string]interface{}); ok {
			if h, ok := spk["hex"].(string); ok {
				out["reused"] = walletScriptReused(paths, h)
			}
		}
	}
	if desc := walletDescriptorForAddress(chainName, paths, addr); desc != "" {
		out["desc"] = desc
	}
	if paths.WalletAddressInReceiveKeypool != nil && paths.WalletAddressInReceiveKeypool(addr) {
		out["iskeypool"] = true
	}
	if paths.WalletAddressInChangeKeypool != nil && paths.WalletAddressInChangeKeypool(addr) {
		out["iskeypool"] = true
	}
	if paths.WalletAddressIsNodeTip != nil && paths.WalletAddressIsNodeTip(addr) {
		out["isnodetip"] = true
	}
	if paths.WalletAddressCorePoolIndex != nil {
		if coreIdx, ok := paths.WalletAddressCorePoolIndex(addr); ok {
			out["hd_keypool_core_index"] = coreIdx
		}
	}
}

// walletAddressLabelPurpose returns Core getaddressesbylabel / labels purpose ("receive" or "send").
func walletAddressLabelPurpose(paths *DataPaths, addr string) string {
	if paths != nil && paths.WalletAddressHDPath != nil {
		if _, ischg, ok := paths.WalletAddressHDPath(addr); ok && ischg {
			return "send"
		}
	}
	return "receive"
}

func walletScriptReused(paths *DataPaths, scriptHex string) bool {
	if paths == nil || paths.WalletIsScriptReused == nil {
		return false
	}
	scriptHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(scriptHex), "0x"))
	pk, err := hex.DecodeString(scriptHex)
	if err != nil || len(pk) == 0 {
		return false
	}
	return paths.WalletIsScriptReused(pk)
}
