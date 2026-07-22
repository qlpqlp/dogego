// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"testing"

	"dogego/wire"
)

func TestUtxoCacheApplyAndLookup(t *testing.T) {
	u := NewUtxoCache()
	funding := wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{}},
		Vout: []wire.TxOut{
			{Value: 5e8, PkScript: []byte{0x76, 0xa9, 0x14, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0x88, 0xac}},
		},
	}
	spend := wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 4e8, PkScript: []byte{0x76, 0xa9}}},
	}
	gen := wire.ParsedBlock{Txs: []*wire.Tx{{Version: 1, Vin: []wire.TxIn{{}}, Vout: []wire.TxOut{{Value: 50e8, PkScript: []byte{0x00}}}}}}
	blk1 := wire.ParsedBlock{Txs: []*wire.Tx{&funding}}
	blk2 := wire.ParsedBlock{Txs: []*wire.Tx{&spend}}

	if err := u.ApplyBlock(&gen, 0); err != nil {
		t.Fatal(err)
	}
	if err := u.ApplyBlock(&blk1, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := u.LookupOutpoint(funding.TxHash(), 0); !ok {
		t.Fatal("expected funding output in cache")
	}
	if err := u.ApplyBlock(&blk2, 2); err != nil {
		t.Fatal(err)
	}
	if _, ok := u.LookupOutpoint(funding.TxHash(), 0); ok {
		t.Fatal("expected funding output spent")
	}
	if u.TipHeight() != 2 {
		t.Fatalf("tip %d", u.TipHeight())
	}
}

func TestUtxoCacheIdempotentHeight(t *testing.T) {
	u := NewUtxoCache()
	gen := wire.ParsedBlock{Txs: []*wire.Tx{{Version: 1, Vin: []wire.TxIn{{}}, Vout: []wire.TxOut{{Value: 50e8}}}}}
	if err := u.ApplyBlock(&gen, 0); err != nil {
		t.Fatal(err)
	}
	if err := u.ApplyBlock(&gen, 0); err != nil {
		t.Fatal(err)
	}
	if u.Count() != 1 {
		t.Fatalf("count %d", u.Count())
	}
}
