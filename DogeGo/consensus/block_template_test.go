// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/mempool"
	"dogego/wire"
)

func TestSelectBlockTemplateTxsParentChild(t *testing.T) {
	var prev [32]byte
	prev[0] = 9
	view := stubFeeView{outpointKey(prev, 0): {Value: 10_000, PkScript: []byte{0x51}}}
	pool := mempool.New(10)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 9000, PkScript: []byte{0x51}}},
	}
	child := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parent.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 8000, PkScript: []byte{0x52}}},
	}
	pr, _ := parent.Serialize()
	cr, _ := child.Serialize()
	_ = pool.Add(pr)
	_ = pool.Add(cr)
	sel, err := SelectBlockTemplateTxs(pool, view, MaxBlockWeight)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Txs) != 2 {
		t.Fatalf("selected %d want 2", len(sel.Txs))
	}
	if sel.TotalFees != 2000 {
		t.Fatalf("fees %d want 2000", sel.TotalFees)
	}
	if len(sel.Txs[1].Depends) != 1 || sel.Txs[1].Depends[0] != 1 {
		t.Fatalf("child depends %#v", sel.Txs[1].Depends)
	}
	if sel.Txs[0].Data == "" || sel.Txs[0].Fee != 1000 {
		t.Fatalf("parent entry %#v", sel.Txs[0])
	}
}

func TestSelectBlockTemplateTxsPrioritisedChildBoostsParent(t *testing.T) {
	var prev [32]byte
	prev[0] = 8
	view := stubFeeView{outpointKey(prev, 0): {Value: 10_000, PkScript: []byte{0x51}}}
	pool := mempool.New(10)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 9000, PkScript: []byte{0x51}}},
	}
	child := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parent.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 8990, PkScript: []byte{0x52}}},
	}
	pr, _ := parent.Serialize()
	cr, _ := child.Serialize()
	_ = pool.Add(pr)
	_ = pool.Add(cr)
	childID := txidDisplayFromLE(child.TxHash())
	if err := pool.AddFeeDelta(childID, 100_000_000); err != nil {
		t.Fatal(err)
	}
	sel, err := SelectBlockTemplateTxs(pool, view, MaxBlockWeight)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Txs) != 2 {
		t.Fatalf("selected %d want 2 (prioritised child should pull parent)", len(sel.Txs))
	}
}

func TestSelectBlockTemplateTxsRespectsWeight(t *testing.T) {
	var prev [32]byte
	prev[0] = 1
	view := stubFeeView{outpointKey(prev, 0): {Value: 1_000_000, PkScript: []byte{0x51}}}
	pool := mempool.New(10)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 900_000, PkScript: []byte{0x51}}},
	}
	raw, _ := tx.Serialize()
	_ = pool.Add(raw)
	sel, err := SelectBlockTemplateTxs(pool, view, coinbaseTemplateWeight)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Txs) != 0 {
		t.Fatalf("expected none, got %d", len(sel.Txs))
	}
}
