// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/chain"
	"dogego/wire"
)

func TestCheckTxInputsAtHeightCoinbaseMaturity(t *testing.T) {
	var prev [32]byte
	prev[0] = 0xcb
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	const coinH = int64(200_000)
	src := stubUtxoCoinSource{
		heights:  map[[36]byte]int64{outpointKey(prev, 0): coinH},
		coinbase: map[[36]byte]bool{outpointKey(prev, 0): true},
	}
	view := UtxoPrevOutView{Source: src}
	if err := CheckTxInputsAtHeight(spend, view, coinH+100, chain.RebootTestnet); !errors.Is(err, ErrCoinbaseImmature) {
		t.Fatalf("want immature, got %v", err)
	}
	if err := CheckTxInputsAtHeight(spend, view, coinH+300, chain.RebootTestnet); err != nil {
		t.Fatalf("mature: %v", err)
	}
}
