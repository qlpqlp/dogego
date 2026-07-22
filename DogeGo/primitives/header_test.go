// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package primitives_test

import (
	"testing"

	"dogego/primitives"
)

func TestBlockHeaderRoundTrip(t *testing.T) {
	var h primitives.BlockHeader
	h.Version = 2
	for i := range h.PrevBlock {
		h.PrevBlock[i] = byte(i)
		h.MerkleRoot[i] = byte(255 - i)
	}
	h.Timestamp = 1234567890
	h.Bits = 0x1e0ffff0
	h.Nonce = 42
	w := h.EncodeWire80()
	var h2 primitives.BlockHeader
	if err := h2.DecodeWire80(w[:]); err != nil {
		t.Fatal(err)
	}
	w2 := h2.EncodeWire80()
	if w != w2 {
		t.Fatalf("mismatch after round-trip")
	}
}
