// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "encoding/json"

// execSignRawTransactionWithWallet signs using the built-in wallet key and UTXO prevouts.
func execSignRawTransactionWithWallet(chainName string, paths *DataPaths, params []json.RawMessage) (map[string]interface{}, int, string) {
	if len(params) < 1 {
		return nil, -8, "signrawtransactionwithwallet: hex string required"
	}
	if rpcWalletAddress(paths) == "" {
		return nil, -1, "signrawtransactionwithwallet: wallet is not implemented in DogeGo"
	}
	if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
		return nil, code, msg
	}
	trimmed := make([]json.RawMessage, 0, 4)
	trimmed = append(trimmed, params[0])
	if len(params) > 1 {
		trimmed = append(trimmed, params[1])
	}
	if len(params) > 2 {
		trimmed = append(trimmed, params[2])
	}
	if len(params) > 3 {
		return nil, -32602, "Too many arguments"
	}
	return execSignRawTransaction(chainName, paths, trimmed)
}
