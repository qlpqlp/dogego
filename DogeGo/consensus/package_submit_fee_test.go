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

func TestPackageTxsFeeSizeCPFP(t *testing.T) {
	pk := []byte{0x51}
	confirmed := [32]byte{0x11}
	view := stubPrevOutView{}
	var k [36]byte
	copy(k[:32], confirmed[:])
	view[k] = PrevOut{Value: 1_000_000_000, PkScript: pk}

	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: confirmed, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 999_999_000, PkScript: pk}}, // fee 1_000 (below min for tiny tx)
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 999_000_000, PkScript: pk}}, // fee 999_000
	}
	fee, size, err := PackageTxsFeeSize([]*wire.Tx{parent, child}, view)
	if err != nil {
		t.Fatal(err)
	}
	if fee != 1_000_000 {
		t.Fatalf("package fee=%d want 1000000", fee)
	}
	if size <= 0 {
		t.Fatal("size")
	}
	if err := CheckMinRelayFeePackageTxs([]*wire.Tx{parent, child}, view, DefaultMinRelayTxFeePerKB); err != nil {
		t.Fatalf("package should meet min relay: %v", err)
	}
	if err := CheckMinRelayFee(parent, view, DefaultMinRelayTxFeePerKB); !errors.Is(err, ErrMinRelayFee) {
		t.Fatalf("parent alone should fail min relay, got %v", err)
	}
}

func TestCheckMinRelayFeePackageTxsRejectsUnderpay(t *testing.T) {
	pk := []byte{0x51}
	confirmed := [32]byte{0x22}
	view := stubPrevOutView{}
	var k [36]byte
	copy(k[:32], confirmed[:])
	view[k] = PrevOut{Value: 1_000_000_000, PkScript: pk}

	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: confirmed, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 999_999_999, PkScript: pk}}, // fee 1
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 999_999_998, PkScript: pk}}, // fee 1
	}
	if err := CheckMinRelayFeePackageTxs([]*wire.Tx{parent, child}, view, DefaultMinRelayTxFeePerKB); !errors.Is(err, ErrMinRelayFee) {
		t.Fatalf("expected ErrMinRelayFee, got %v", err)
	}
}
