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

type stubPrevOutView map[[36]byte]PrevOut

func (s stubPrevOutView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	var k [36]byte
	copy(k[:32], prevHash[:])
	k[32] = byte(idx)
	k[33] = byte(idx >> 8)
	k[34] = byte(idx >> 16)
	k[35] = byte(idx >> 24)
	o, ok := s[k]
	return o, ok
}

func TestFeeForSizeAndMinRelay(t *testing.T) {
	if FeeForSize(DefaultMinRelayTxFeePerKB, 500) != 50_000 {
		t.Fatalf("fee for 500 bytes: got %d", FeeForSize(DefaultMinRelayTxFeePerKB, 500))
	}
	if FeeForSize(1000, 1) != 1 {
		t.Fatalf("round up to 1 koinu when rate*size/1000 is zero")
	}
	var prev [32]byte
	prev[0] = 1
	view := stubPrevOutView{}
	view[outpointKey(prev, 0)] = PrevOut{Value: 200_000, PkScript: []byte{0x51}}

	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 50_000, PkScript: []byte{0x51}}},
	}
	fee, err := TxFee(tx, view)
	if err != nil || fee != 150_000 {
		t.Fatalf("TxFee: fee=%d err=%v", fee, err)
	}
	if err := CheckMinRelayFee(tx, view, DefaultMinRelayTxFeePerKB); err != nil {
		t.Fatalf("high fee tx rejected: %v", err)
	}
	tx.Vout[0].Value = 199_000
	if err := CheckMinRelayFee(tx, view, DefaultMinRelayTxFeePerKB); !errors.Is(err, ErrMinRelayFee) {
		t.Fatalf("want ErrMinRelayFee, got %v", err)
	}
}

func TestEffectiveMinRelayFeePerKB(t *testing.T) {
	if EffectiveMinRelayFeePerKB(0, 0) != DefaultMinRelayTxFeePerKB {
		t.Fatal()
	}
	if EffectiveMinRelayFeePerKB(200_000, 0) != 200_000 {
		t.Fatal()
	}
	if EffectiveMinRelayFeePerKB(0, 300_000) != 300_000 {
		t.Fatal()
	}
}
