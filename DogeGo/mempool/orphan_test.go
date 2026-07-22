// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"testing"

	"dogego/wire"
)

func TestOrphanPoolChildrenOf(t *testing.T) {
	o := NewOrphanPool(10)
	parentHash := [32]byte{1}
	child := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parentHash,
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	raw, _ := child.Serialize()
	pid := txidDisplayHex(parentHash)
	if _, err := o.Add(raw, []string{pid}, ""); err != nil {
		t.Fatal(err)
	}
	kids := o.ChildrenOf(pid)
	if len(kids) != 1 {
		t.Fatalf("children %d", len(kids))
	}
	o.Remove(txidDisplayHex(child.TxHash()))
	if len(o.ChildrenOf(pid)) != 0 {
		t.Fatal("expected empty after remove")
	}
}

func TestOrphanPoolEvictsWhenFull(t *testing.T) {
	o := NewOrphanPool(2)
	mk := func(tag byte) ([]byte, string) {
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{{
				PrevHash: [32]byte{tag},
				PrevIdx:  0,
				Sequence: 0xffffffff,
			}},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		raw, _ := tx.Serialize()
		return raw, txidDisplayHex([32]byte{tag})
	}
	r1, p1 := mk(1)
	if _, err := o.Add(r1, []string{p1}, ""); err != nil {
		t.Fatal(err)
	}
	r2, p2 := mk(2)
	if _, err := o.Add(r2, []string{p2}, ""); err != nil {
		t.Fatal(err)
	}
	r3, p3 := mk(3)
	if _, err := o.Add(r3, []string{p3}, ""); err != nil {
		t.Fatal(err)
	}
	if o.Count() != 2 {
		t.Fatalf("count %d", o.Count())
	}
}

func TestOrphanRemoveByPeer(t *testing.T) {
	o := NewOrphanPool(10)
	parentHash := [32]byte{2}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	raw, _ := tx.Serialize()
	if _, err := o.Add(raw, []string{txidDisplayHex(parentHash)}, "1.2.3.4:8333"); err != nil {
		t.Fatal(err)
	}
	if n := o.RemoveByPeer("1.2.3.4:8333"); n != 1 {
		t.Fatalf("removed %d want 1", n)
	}
	if o.Count() != 0 {
		t.Fatalf("count %d", o.Count())
	}
}
