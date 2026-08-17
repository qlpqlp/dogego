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

type maturityJournal struct {
	coinHeight int64
	tip        int64
}

func (m *maturityJournal) TipHeight() (int64, error)         { return m.tip, nil }
func (m *maturityJournal) ReadHeaderAt(int64) ([]byte, error) { return make([]byte, 80), nil }
func (m *maturityJournal) HeightByDisplayHash(string) (int64, error) {
	return m.coinHeight, nil
}

type maturityIndex struct {
	coinbaseID [32]byte
}

func (m maturityIndex) Lookup(txidHex string) ([32]byte, uint32, error) {
	if txidDisplayFromLE(m.coinbaseID) == txidHex {
		return [32]byte{0xaa}, 0, nil
	}
	return [32]byte{}, 0, errors.New("missing")
}

func TestCheckTxCoinbaseMaturity(t *testing.T) {
	var coinID [32]byte
	coinID[0] = 0xcb
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: coinID, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	ix := maturityIndex{coinbaseID: coinID}
	const coinH = int64(200_000) // post-Digishield (240-block maturity)
	j := &maturityJournal{coinHeight: coinH, tip: coinH + 500}
	if err := CheckTxCoinbaseMaturity(spend, coinH+100, chain.RebootTestnet, ix, j); !errors.Is(err, ErrCoinbaseImmature) {
		t.Fatalf("want immature, got %v", err)
	}
	if err := CheckTxCoinbaseMaturity(spend, coinH+300, chain.RebootTestnet, ix, j); err != nil {
		t.Fatalf("mature: %v", err)
	}
}

func TestCheckTxCoinbaseMaturityFromViewSkipsIndexWhenMature(t *testing.T) {
	var prev [32]byte
	prev[0] = 0x11
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	src := stubUtxoHeightSource{heights: map[[36]byte]int64{outpointKey(prev, 0): 10}}
	view := UtxoPrevOutView{Source: src}
	if err := CheckTxCoinbaseMaturityFromView(spend, 10+300, chain.RebootTestnet, view, panicIndexer{}, nil); err != nil {
		t.Fatalf("mature UTXO height must not consult txindex: %v", err)
	}
}
