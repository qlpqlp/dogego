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

func TestMempoolAncestorAndDescendantChain(t *testing.T) {
	p := New(10)
	parentRaw := minimalCoinbaseRaw(t)
	if err := p.Add(parentRaw); err != nil {
		t.Fatal(err)
	}
	parentTx, err := wire.DeserializeTx(parentRaw)
	if err != nil {
		t.Fatal(err)
	}
	parentID := txidDisplayHex(parentTx.TxHash())

	child := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: parentTx.TxHash(),
			PrevIdx:  0,
			Script:   []byte{0x51},
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{
			Value:    1000,
			PkScript: []byte{0x51},
		}},
		LockTime: 0,
	}
	childRaw, err := child.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Add(childRaw); err != nil {
		t.Fatal(err)
	}
	childTx, err := wire.DeserializeTx(childRaw)
	if err != nil {
		t.Fatal(err)
	}
	childID := txidDisplayHex(childTx.TxHash())

	anc, err := p.MempoolAncestorTxIDs(childID)
	if err != nil {
		t.Fatal(err)
	}
	if len(anc) != 1 || anc[0] != parentID {
		t.Fatalf("ancestors %v want [%s]", anc, parentID)
	}
	desc, err := p.MempoolDescendantTxIDs(parentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(desc) != 1 || desc[0] != childID {
		t.Fatalf("descendants %v want [%s]", desc, childID)
	}
	if _, err := p.MempoolAncestorTxIDs(parentID); err != nil {
		t.Fatal(err)
	}
	if a, _ := p.MempoolAncestorTxIDs(parentID); len(a) != 0 {
		t.Fatalf("coinbase ancestors %v", a)
	}
	if d, _ := p.MempoolDescendantTxIDs(childID); len(d) != 0 {
		t.Fatalf("leaf descendants %v", d)
	}
}

func TestMempoolAncestorUnknownTx(t *testing.T) {
	p := New(10)
	_, err := p.MempoolAncestorTxIDs("0000000000000000000000000000000000000000000000000000000000000001")
	if err == nil {
		t.Fatal("expected error")
	}
}
