// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import "testing"

func TestNewPsbtFromTx(t *testing.T) {
	tx := &Tx{
		Version: 2,
		Vin:     []TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: MaxBIP125RBFSequence}},
		Vout:    []TxOut{{Value: 100, PkScript: []byte{0x51}}},
	}
	p, err := NewPsbtFromTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Inputs) != 1 || len(p.Outputs) != 1 {
		t.Fatalf("in=%d out=%d", len(p.Inputs), len(p.Outputs))
	}
	raw, err := p.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := ParsePSBT(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p2.UnsignedTx.Version != 2 {
		t.Fatalf("version %d", p2.UnsignedTx.Version)
	}
}
