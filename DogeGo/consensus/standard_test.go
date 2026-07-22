// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/wire"
)

func TestIsStandardTxRejectsDust(t *testing.T) {
	pkScript := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	pkScript = append(pkScript, 0x88, 0xac)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: HardDustLimitKoinu - 1, PkScript: pkScript}},
	}
	err := IsStandardTx(tx, DefaultStandardPolicy(), 0)
	if !errors.Is(err, ErrNonStandardTx) {
		t.Fatalf("got %v", err)
	}
}

func TestIsStandardTxAllowsAboveDust(t *testing.T) {
	pkScript := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	pkScript = append(pkScript, 0x88, 0xac)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: HardDustLimitKoinu, PkScript: pkScript}},
	}
	if err := IsStandardTx(tx, DefaultStandardPolicy(), 0); err != nil {
		t.Fatal(err)
	}
}

func TestIsStandardTxRejectsMultiOpReturn(t *testing.T) {
	null := []byte{0x6a, 0x04, 0xde, 0xad, 0xbe, 0xef}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 0, PkScript: null},
			{Value: 0, PkScript: null},
		},
	}
	err := IsStandardTx(tx, DefaultStandardPolicy(), 0)
	if !errors.Is(err, ErrNonStandardTx) {
		t.Fatalf("got %v", err)
	}
}

func TestIsStandardTxRejectsNonZeroOpReturn(t *testing.T) {
	null := []byte{0x6a, 0x04, 0xde, 0xad, 0xbe, 0xef}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: null}},
	}
	err := IsStandardTx(tx, DefaultStandardPolicy(), 0)
	if !errors.Is(err, ErrNonStandardTx) {
		t.Fatalf("got %v", err)
	}
}
