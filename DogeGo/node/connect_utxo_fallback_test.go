// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/consensus"
	"dogego/store"
	"dogego/wire"
)

// Regression: connect catch-up must not fail sequence-lock resolution when txindex lags UTXO (mainnet ~6857).
func TestConnectPrevHeightsUseUtxoWhenTxIndexMisses(t *testing.T) {
	var prev [32]byte
	prev[0] = 0x42
	src := &utxoHeightTestSource{height: 9, prev: prev}
	view := consensus.MultiPrevOutView{
		consensus.UtxoPrevOutView{Source: src},
	}
	tx := &wire.Tx{Vin: []wire.TxIn{{PrevHash: prev, PrevIdx: 0}}}
	got, err := consensus.PrevHeightsForTx(tx, nil, nil, 10, nil, 0, view)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 9 {
		t.Fatalf("heights %v want [9]", got)
	}
}

type utxoHeightTestSource struct {
	height int64
	prev   [32]byte
}

func (u *utxoHeightTestSource) UnspentOutpoint(prevHash [32]byte, vout uint32) (int64, []byte, bool) {
	if prevHash != u.prev || vout != 0 {
		return 0, nil, false
	}
	return 1, []byte{0x76, 0xa9}, true
}

func (u *utxoHeightTestSource) UnspentHeight(prevHash [32]byte, vout uint32) (int64, bool) {
	if prevHash != u.prev || vout != 0 {
		return 0, false
	}
	return u.height, true
}

func TestConnectCatchUpLagMatchesContiguousMinusUtxo(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{Params: p, contiguousTip: 100, Utxo: store.NewUtxoCache()}
	bs.Utxo.SetTipHeightForTest(40)
	if lag := ConnectCatchUpLag(bs, bs.Utxo); lag != 60 {
		t.Fatalf("lag=%d want 60", lag)
	}
}
