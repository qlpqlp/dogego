// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "dogego/store"

// utxoRowSpendableForFund rejects immature coinbase outputs for wallet coin selection (Core spendable).
func utxoRowSpendableForFund(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex, row store.UtxoDumpRow, chainName string) bool {
	if paths == nil || paths.Utxo == nil || row.Height < 0 {
		return true
	}
	tip := paths.Utxo.TipHeight()
	if tip < 0 {
		return true
	}
	conf := tip - row.Height + 1
	if conf < walletCoinbaseMaturity(chainName, j, raw, paths) && walletUtxoImmatureCoinbase(row, ix, raw) {
		return false
	}
	return true
}
