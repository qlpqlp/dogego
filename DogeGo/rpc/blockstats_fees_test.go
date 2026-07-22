// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/consensus"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestBlockStatsPrevOutViewUtxoParent(t *testing.T) {
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: []byte{0x51}}},
	}
	pb0 := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	if err := utxo.ApplyBlock(pb0, 0); err != nil {
		t.Fatal(err)
	}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: coin.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 4_900_000_000, PkScript: []byte{0x51}}},
	}
	pb1 := &wire.ParsedBlock{
		Header: primitives.BlockHeader{Version: 1, Timestamp: 2},
		Txs: []*wire.Tx{
			{Version: 1, Vin: []wire.TxIn{{PrevIdx: 0xffffffff}}, Vout: []wire.TxOut{{Value: 0, PkScript: []byte{0x51}}}},
			spend,
		},
	}
	view := blockStatsPrevOutView(0, utxo, nil, nil)
	if view == nil {
		t.Fatal("nil view")
	}
	st, ok := consensus.ComputeBlockFeeStats(pb1, view)
	if !ok || st.TotalFee != 100_000_000 {
		t.Fatalf("ok=%v %+v", ok, st)
	}
}
