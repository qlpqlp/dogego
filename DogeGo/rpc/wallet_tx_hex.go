// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"

	"dogego/wire"
)

func walletRecordTxHex(paths *DataPaths, txid, hexStr string) {
	if paths == nil || paths.WalletRememberTxHex == nil {
		return
	}
	_ = paths.WalletRememberTxHex(txid, hexStr)
}

// walletTxSpendsFromWallet reports whether tx spends a UTXO owned by HD spend scripts.
func walletTxSpendsFromWallet(paths *DataPaths, tx *wire.Tx) bool {
	if paths == nil || tx == nil || paths.Utxo == nil {
		return false
	}
	spendSet := walletSpendScriptSet(paths)
	if len(spendSet) == 0 {
		return false
	}
	for _, in := range tx.Vin {
		if isCoinbaseWireIn(&in) {
			continue
		}
		e, ok := paths.Utxo.LookupOutpoint(in.PrevHash, in.PrevIdx)
		if !ok {
			continue
		}
		if _, mine := spendSet[hex.EncodeToString(e.PkScript)]; mine {
			return true
		}
	}
	return false
}
