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

func TestParsePSBTMinimal(t *testing.T) {
	tx := &Tx{
		Version: 1,
		Vin: []TxIn{{
			PrevHash: [32]byte{1},
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []TxOut{{
			Value:    100_000_000,
			PkScript: []byte{0x76, 0xa9, 0x14, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x88, 0xac},
		}},
		LockTime: 0,
	}
	unsigned := tx.SerializeForHash()
	var b bytes.Buffer
	b.Write(psbtMagic)
	writeKV(&b, []byte{PsbtGlobalUnsignedTx}, unsigned)
	b.WriteByte(0) // end global
	b.WriteByte(0) // end input 0
	b.WriteByte(0) // end output 0
	p, err := ParsePSBT(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if p.UnsignedTx == nil || len(p.UnsignedTx.Vin) != 1 || len(p.UnsignedTx.Vout) != 1 {
		t.Fatalf("tx %+v", p.UnsignedTx)
	}
	if len(p.Inputs) != 1 || len(p.Outputs) != 1 {
		t.Fatalf("maps in=%d out=%d", len(p.Inputs), len(p.Outputs))
	}
}

func writeKV(b *bytes.Buffer, key, val []byte) {
	_ = WriteCompactSize(b, uint64(len(key)))
	_, _ = b.Write(key)
	_ = WriteCompactSize(b, uint64(len(val)))
	_, _ = b.Write(val)
}

func TestParsePSBTWithPartialSig(t *testing.T) {
	tx := &Tx{
		Version: 1,
		Vin: []TxIn{{
			PrevHash: [32]byte{2},
			PrevIdx:  1,
			Sequence: 0xfffffffe,
		}},
		Vout: []TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	unsigned := tx.SerializeForHash()
	pub := make([]byte, 33)
	pub[0] = 0x02
	sig := make([]byte, 71)
	sig[0] = 0x30
	var b bytes.Buffer
	b.Write(psbtMagic)
	writeKV(&b, []byte{PsbtGlobalUnsignedTx}, unsigned)
	b.WriteByte(0)
	key := append([]byte{PsbtInPartialSig}, pub...)
	writeKV(&b, key, sig)
	b.WriteByte(0)
	b.WriteByte(0)
	p, err := ParsePSBT(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Inputs[0]) != 1 || p.Inputs[0][0].Type != PsbtInPartialSig {
		t.Fatalf("input map %#v", p.Inputs[0])
	}
}
