// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"testing"
)

func TestJoinPSBT(t *testing.T) {
	tx1 := &Tx{
		Version: 1,
		Vin:     []TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []TxOut{{Value: 1e8, PkScript: []byte{0x51}}},
	}
	tx2 := &Tx{
		Version: 2,
		Vin:     []TxIn{{PrevHash: [32]byte{2}, PrevIdx: 1, Sequence: 0xfffffffe}},
		Vout:    []TxOut{{Value: 2e8, PkScript: []byte{0x52}}},
		LockTime: 100,
	}
	p1, err := NewPsbtFromTx(tx1)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := NewPsbtFromTx(tx2)
	if err != nil {
		t.Fatal(err)
	}
	joined, err := JoinPSBT([]*Psbt{p1, p2})
	if err != nil {
		t.Fatal(err)
	}
	if len(joined.UnsignedTx.Vin) != 2 || len(joined.UnsignedTx.Vout) != 2 {
		t.Fatalf("tx %#v", joined.UnsignedTx)
	}
	if joined.UnsignedTx.Version != 2 {
		t.Fatalf("version %d", joined.UnsignedTx.Version)
	}
	if joined.UnsignedTx.LockTime != 100 {
		t.Fatalf("locktime %d", joined.UnsignedTx.LockTime)
	}
	_, err = JoinPSBT([]*Psbt{p1, p1})
	if err == nil {
		t.Fatal("expected duplicate input error")
	}
}
