// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

// scryptVectorHeader80 is a known-valid 80-byte header from pow/scrypt_vector_test.go.
func scryptVectorHeader80(t *testing.T) []byte {
	t.Helper()
	in, err := hex.DecodeString("020000004c1271c211717198227392b029a64a7971931d351b387bb80db027f270411e398a07046f7d4a08dd815412a8712f874a7ebf0507e3878bd24e20a3b73fd750a667d2f451eac7471b00de6659")
	if err != nil || len(in) != 80 {
		t.Fatal(err)
	}
	return in
}

func TestCheckAuxPowRejectsInsufficientParentPoW(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	// Child nBits is in the 80-byte header hash; set target before building the coinbase commitment.
	binary.LittleEndian.PutUint32(child[72:76], 0x01000100)
	wireAuxPowCoinbaseScript(a, child)
	err := checkAuxPow(child, a, dc)
	if err == nil || !contains(err.Error(), "aux parent pow") {
		t.Fatalf("err=%v", err)
	}
}

func TestScryptVectorHeaderMeetsItsBits(t *testing.T) {
	h := scryptVectorHeader80(t)
	bits := binary.LittleEndian.Uint32(h[72:76])
	if err := pow.CheckProofOfWorkLE(pow.ScryptHashLE(h), bits, PowLimitHex); err != nil {
		t.Fatalf("vector header should meet its nBits: %v", err)
	}
}
