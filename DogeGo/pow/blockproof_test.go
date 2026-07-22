// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow_test

import (
	"encoding/binary"
	"math/big"
	"testing"

	"dogego/pow"
)

func TestBlockProofFromBitsTwiceEqualsDouble(t *testing.T) {
	bits := uint32(0x1e0ffff0)
	w, err := pow.BlockProofFromBits(bits)
	if err != nil {
		t.Fatal(err)
	}
	if w.Sign() <= 0 {
		t.Fatalf("expected positive work got %v", w)
	}
	sum := new(big.Int).Add(w, w)
	got, err := cumulativeFromHeaders([]uint32{bits, bits})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmp(sum) != 0 {
		t.Fatalf("cumulative %v want %v", got, sum)
	}
}

func cumulativeFromHeaders(bitsList []uint32) (*big.Int, error) {
	var sum big.Int
	for _, bits := range bitsList {
		w, err := pow.BlockProofFromBits(bits)
		if err != nil {
			return nil, err
		}
		sum.Add(&sum, w)
	}
	return &sum, nil
}

func TestBlockProofZeroBits(t *testing.T) {
	w, err := pow.BlockProofFromBits(0)
	if err != nil {
		t.Fatal(err)
	}
	if w.Sign() != 0 {
		t.Fatalf("want 0 got %v", w)
	}
}

func TestBlockProofFromGenesisLikeHeader(t *testing.T) {
	var h80 [80]byte
	binary.LittleEndian.PutUint32(h80[72:76], 0x1e0ffff0)
	bits := binary.LittleEndian.Uint32(h80[72:76])
	w, err := pow.BlockProofFromBits(bits)
	if err != nil {
		t.Fatal(err)
	}
	if pow.ChainworkHex(w) == "0" {
		t.Fatal("expected non-zero chainwork hex")
	}
}
