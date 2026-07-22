// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"math"
)

func rpcWalletPayTxFee(paths *DataPaths) float64 {
	if paths == nil || paths.WalletPayTxFee == nil {
		return 0
	}
	return paths.WalletPayTxFee()
}

func rpcWalletPayTxFeeKoinuPerKB(paths *DataPaths) uint64 {
	f := rpcWalletPayTxFee(paths)
	if f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return uint64(math.Round(f * 1e8))
}

// execSetTxFee persists the wallet fee rate (DOGE per kB) when the built-in wallet is enabled.
func execSetTxFee(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	fee, code, msg := parseSetTxFeeAmount(params[0])
	if code != 0 {
		return nil, code, msg
	}
	if paths == nil || paths.WalletSetPayTxFee == nil || rpcWalletAddress(paths) == "" {
		return true, 0, ""
	}
	if err := paths.WalletSetPayTxFee(fee); err != nil {
		return nil, -1, "settxfee: " + err.Error()
	}
	return true, 0, ""
}
