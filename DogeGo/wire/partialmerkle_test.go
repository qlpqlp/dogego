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

func TestPartialMerkleRoundTripOneTx(t *testing.T) {
	var h0 [32]byte
	h0[0] = 0xab
	vTxid := [][32]byte{h0}
	match := []bool{true}
	pmt, err := NewPartialMerkleTree(vTxid, match)
	if err != nil {
		t.Fatal(err)
	}
	root, m, _, ok := pmt.ExtractMatches()
	if !ok || len(m) != 1 || m[0] != h0 {
		t.Fatalf("extract: ok=%v m=%x root=%x", ok, m, root)
	}
	var hdr [80]byte
	copy(hdr[36:68], root[:])
	proof, err := SerializeMerkleBlock(hdr[:], pmt)
	if err != nil {
		t.Fatal(err)
	}
	h2, p2, err := ParseMerkleBlockProof(proof)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(h2, hdr[:]) {
		t.Fatal("header mismatch")
	}
	r2, m2, _, ok2 := p2.ExtractMatches()
	if !ok2 || r2 != root || len(m2) != 1 || m2[0] != h0 {
		t.Fatalf("roundtrip extract r2=%x m2=%v", r2, m2)
	}
}
