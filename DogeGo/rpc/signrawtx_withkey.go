// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "encoding/json"

// execSignRawTransactionWithKey signs with explicit WIF keys only (Core signrawtransactionwithkey).
func execSignRawTransactionWithKey(chainName string, paths *DataPaths, params []json.RawMessage) (map[string]interface{}, int, string) {
	if len(params) < 1 {
		return nil, -8, "signrawtransactionwithkey: hex string required"
	}
	if len(params) > 4 {
		return nil, -32602, "Too many arguments"
	}
	return execSignRawTransaction(chainName, paths, params, signRawTxOpts{keysOnly: true})
}
