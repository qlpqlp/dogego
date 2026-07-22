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

func TestSerializeAuxPowRoundTrip(t *testing.T) {
	inner := []byte{0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0x01, 0x00, 0xff, 0xff, 0xff, 0xff, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x51, 0x00, 0x00, 0x00, 0x00}
	tx, err := ReadTx(bytes.NewReader(inner))
	if err != nil {
		t.Fatal(err)
	}
	orig := &AuxPow{Coinbase: tx, MerkleIndex: 0, ChainIndex: 0}
	raw, err := SerializeAuxPow(orig)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadAuxPow(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got.MerkleIndex != orig.MerkleIndex || got.ChainIndex != orig.ChainIndex {
		t.Fatalf("index mismatch")
	}
}
