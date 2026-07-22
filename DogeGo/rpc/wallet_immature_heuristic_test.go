// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/store"
)

func TestWalletUtxoImmatureCoinbaseCompactIndexHeuristic(t *testing.T) {
	ix := &store.TxIndex{EmbedTx: false}
	row := store.UtxoDumpRow{TxID: "abc", Vout: 0}
	if !walletUtxoImmatureCoinbase(row, ix, nil) {
		t.Fatal("vout 0 + compact index should treat as coinbase without block load")
	}
	row.Vout = 1
	if walletUtxoImmatureCoinbase(row, ix, nil) {
		t.Fatal("vout 1 without raw store should not be immature coinbase")
	}
}

func TestWalletUtxoImmatureCoinbaseEmbedTxUsesLookup(t *testing.T) {
	ix := &store.TxIndex{EmbedTx: true}
	row := store.UtxoDumpRow{TxID: "missing", Vout: 0}
	if walletUtxoImmatureCoinbase(row, ix, nil) {
		t.Fatal("embed_tx on with no raw store should not guess coinbase")
	}
}
