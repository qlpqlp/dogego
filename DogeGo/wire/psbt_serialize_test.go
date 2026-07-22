// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"testing"
)

func TestPsbtSerializeRoundtrip(t *testing.T) {
	tx := &Tx{
		Version: 1,
		Vin: []TxIn{{
			PrevHash: [32]byte{7},
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
	}
	var b bytes.Buffer
	b.Write(psbtMagic)
	writeKV(&b, []byte{PsbtGlobalUnsignedTx}, tx.SerializeForHash())
	b.WriteByte(0) // end global
	b.WriteByte(0) // end input 0
	b.WriteByte(0) // end output 0
	p1, err := ParsePSBT(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := p1.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := ParsePSBT(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.UnsignedTx.Vin) != 1 || p2.UnsignedTx.Vout[0].Value != 50e8 {
		t.Fatalf("roundtrip tx %+v", p2.UnsignedTx)
	}
}

func TestCombinePSBTPartialSigs(t *testing.T) {
	tx := &Tx{
		Version: 1,
		Vin:     []TxIn{{PrevHash: [32]byte{8}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	pubA := make([]byte, 33)
	pubA[0] = 0x02
	pubB := make([]byte, 33)
	pubB[0] = 0x03
	sigA := []byte{0x30, 0x05, 0x01, 0x02, 0x03, 0x04, 0x05}
	sigB := []byte{0x30, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a}

	mk := func(pub, sig []byte) *Psbt {
		var b bytes.Buffer
		b.Write(psbtMagic)
		writeKV(&b, []byte{PsbtGlobalUnsignedTx}, tx.SerializeForHash())
		b.WriteByte(0)
		key := append([]byte{PsbtInPartialSig}, pub...)
		writeKV(&b, key, sig)
		b.WriteByte(0)
		b.WriteByte(0)
		p, err := ParsePSBT(b.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	combined, err := CombinePSBT([]*Psbt{mk(pubA, sigA), mk(pubB, sigB)})
	if err != nil {
		t.Fatal(err)
	}
	if len(combined.Inputs[0]) != 2 {
		t.Fatalf("expected two partial sig entries, got %d", len(combined.Inputs[0]))
	}
}
