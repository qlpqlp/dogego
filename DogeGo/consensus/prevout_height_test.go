// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/wire"
)

type stubUtxoHeightSource struct {
	heights map[[36]byte]int64
}

func (s stubUtxoHeightSource) UnspentOutpoint(prevHash [32]byte, vout uint32) (int64, []byte, bool) {
	if _, ok := s.heights[outpointKey(prevHash, vout)]; !ok {
		return 0, nil, false
	}
	return 1, []byte{0x76, 0xa9}, true
}

func (s stubUtxoHeightSource) UnspentHeight(prevHash [32]byte, vout uint32) (int64, bool) {
	h, ok := s.heights[outpointKey(prevHash, vout)]
	return h, ok
}

func TestPrevHeightsForTxUtxoFallback(t *testing.T) {
	var prev [32]byte
	prev[0] = 0xab
	tx := &wire.Tx{
		Vin: []wire.TxIn{{PrevHash: prev, PrevIdx: 0}},
	}
	src := stubUtxoHeightSource{heights: map[[36]byte]int64{
		outpointKey(prev, 0): 42,
	}}
	view := UtxoPrevOutView{Source: src}
	got, err := PrevHeightsForTx(tx, nil, nil, 100, nil, 0, view)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 42 {
		t.Fatalf("heights %v want [42]", got)
	}
}

func TestPrevHeightsForTxUtxoFallbackViaMultiView(t *testing.T) {
	var prev [32]byte
	prev[1] = 0xcd
	tx := &wire.Tx{Vin: []wire.TxIn{{PrevHash: prev, PrevIdx: 1}}}
	src := stubUtxoHeightSource{heights: map[[36]byte]int64{
		outpointKey(prev, 1): 6856,
	}}
	view := MultiPrevOutView{
		UtxoPrevOutView{Source: src},
	}
	got, err := PrevHeightsForTx(tx, nil, nil, 6857, nil, 0, view)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 6856 {
		t.Fatalf("heights %v want [6856]", got)
	}
}

func TestUtxoPrevOutViewUnspentHeight(t *testing.T) {
	var prev [32]byte
	src := stubUtxoHeightSource{heights: map[[36]byte]int64{outpointKey(prev, 0): 7}}
	view := UtxoPrevOutView{Source: src}
	h, ok := view.UnspentHeight(prev, 0)
	if !ok || h != 7 {
		t.Fatalf("height=%d ok=%v want 7 true", h, ok)
	}
}
