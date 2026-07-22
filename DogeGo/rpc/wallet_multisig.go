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
)

// execAddMultisigAddressWallet creates a P2SH multisig address and imports it watch-only.
func execAddMultisigAddressWallet(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 2 || len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) == 3 && strings.TrimSpace(string(params[2])) != "null" {
		if _, code, msg := parseRPCAccountLabel(params[2], "addmultisigaddress", "account"); code != 0 {
			return nil, code, msg
		}
	}
	if paths == nil || paths.WalletImportWatch == nil || rpcWalletAddress(paths) == "" {
		return nil, -1, "addmultisigaddress: wallet is not implemented in DogeGo"
	}
	m, code, msg := execCreateMultisig(chainName, params[:2])
	if code != 0 {
		return nil, code, msg
	}
	addr, _ := m["address"].(string)
	redeemHex, _ := m["redeemScript"].(string)
	if addr == "" || redeemHex == "" {
		return nil, -1, "addmultisigaddress: internal error"
	}
	redeem, err := hex.DecodeString(redeemHex)
	if err != nil || len(redeem) == 0 {
		return nil, -8, "addmultisigaddress: invalid multisig script"
	}
	h := scriptHash160(redeem)
	p2sh := chain.P2SHScriptFromScriptHash(h)
	if err := rpcWalletImportWatchScript(paths, p2sh, redeem); err != nil {
		return nil, -1, "addmultisigaddress: " + err.Error()
	}
	return addr, 0, ""
}
