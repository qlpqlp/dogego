// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"math/big"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

func TestMaxAlternateForkWork(t *testing.T) {
	var forkPrev, incFirst [32]byte
	forkPrev[0] = 0xaa
	incFirst[0] = 0xbb
	altFirst := [32]byte{0xcc}

	makeBatch := func(first [32]byte, bits uint32) []wire.DecodedHeader {
		h := make([]byte, 80)
		copy(h[4:36], forkPrev[:])
		h[76] = first[0]
		setHeaderBits(h, bits)
		return []wire.DecodedHeader{{Header80: h}}
	}

	incBatch := makeBatch(incFirst, testEasyBits)
	altBatch := makeBatch(altFirst, testHardBits)
	// Same fork as incoming - should not count.
	sameBatch := makeBatch(incFirst, 0x1e0fffff)

	maxAlt := maxAlternateForkWork([][]wire.DecodedHeader{sameBatch, altBatch}, forkPrev, incFirst)
	if maxAlt.Sign() <= 0 {
		t.Fatalf("expected alternate work, got %s", maxAlt)
	}
	incWork, err := incomingChainWork(incBatch)
	if err != nil {
		t.Fatal(err)
	}
	altWork, err := incomingChainWork(altBatch)
	if err != nil {
		t.Fatal(err)
	}
	if maxAlt.Cmp(altWork) != 0 {
		t.Fatalf("maxAlt %s want %s", maxAlt, altWork)
	}
	_ = altWork

	// Two blocks on alternate fork beat one easy incoming block.
	alt1Hash := pow.BlockHashLE(altBatch[0].Header80)
	alt2 := make([]byte, 80)
	copy(alt2, altBatch[0].Header80)
	copy(alt2[4:36], alt1Hash[:])
	alt2[76] ^= 0x33
	setHeaderBits(alt2, testHardBits)
	twoAlt := []wire.DecodedHeader{altBatch[0], {Header80: alt2}}
	maxHard := maxAlternateForkWork([][]wire.DecodedHeader{twoAlt}, forkPrev, incFirst)
	if maxHard.Cmp(incWork) <= 0 {
		t.Fatalf("longer alternate should beat incoming %s vs %s", maxHard, incWork)
	}
}

func TestHeadersExtendFork(t *testing.T) {
	var forkPrev [32]byte
	forkPrev[1] = 0x01
	h := make([]byte, 80)
	copy(h[4:36], forkPrev[:])
	if !headersExtendFork([]wire.DecodedHeader{{Header80: h}}, forkPrev) {
		t.Fatal("expected match")
	}
	var bad [32]byte
	if headersExtendFork([]wire.DecodedHeader{{Header80: h}}, bad) {
		t.Fatal("expected mismatch")
	}
	_ = pow.BlockHashLE(h) // ensure compiles
}

func TestEnsureIncomingForkWins_nilPeerMgr(t *testing.T) {
	var pm *PeerMgr
	if err := pm.EnsureIncomingForkWins(nil, nil, chain.Params{}, 0, [32]byte{}, nil, big.NewInt(1)); err != nil {
		t.Fatal(err)
	}
}

// TestForkElectionMultiBranchStorm picks the strongest alternate among four peer forks (STANDALONE §1 reorg election).
func TestForkElectionMultiBranchStorm(t *testing.T) {
	var forkPrev [32]byte
	forkPrev[2] = 0xcc

	makeFork := func(nonce byte, bits uint32, depth int) []wire.DecodedHeader {
		h := make([]byte, 80)
		copy(h[4:36], forkPrev[:])
		h[76] = nonce
		setHeaderBits(h, bits)
		out := []wire.DecodedHeader{{Header80: append([]byte(nil), h...)}}
		prev := pow.BlockHashLE(h)
		for i := 1; i < depth; i++ {
			next := make([]byte, 80)
			copy(next[4:36], prev[:])
			next[76] = nonce + byte(i)
			setHeaderBits(next, bits)
			out = append(out, wire.DecodedHeader{Header80: next})
			prev = pow.BlockHashLE(next)
		}
		return out
	}

	incoming := makeFork(0x11, testEasyBits, 1)
	inWork, err := incomingChainWork(incoming)
	if err != nil {
		t.Fatal(err)
	}
	inFirst := pow.BlockHashLE(incoming[0].Header80)

	peerBatches := [][]wire.DecodedHeader{
		makeFork(0x22, testEasyBits, 2),   // longer easy
		makeFork(0x33, testHardBits, 1),     // one hard
		makeFork(0x44, testHardBits, 3),     // longest hard - should win alternates
		makeFork(0x55, testEasyBits, 1),     // weak
	}
	maxAlt := maxAlternateForkWork(peerBatches, forkPrev, inFirst)
	wantAlt, err := incomingChainWork(peerBatches[2])
	if err != nil {
		t.Fatal(err)
	}
	if maxAlt.Cmp(wantAlt) != 0 {
		t.Fatalf("maxAlt %s want %s", maxAlt, wantAlt)
	}
	if maxAlt.Cmp(inWork) <= 0 {
		t.Fatalf("strongest alternate should beat easy incoming %s vs %s", maxAlt, inWork)
	}

	longIncoming := makeFork(0x66, testHardBits, 4)
	longWork, err := incomingChainWork(longIncoming)
	if err != nil {
		t.Fatal(err)
	}
	longFirst := pow.BlockHashLE(longIncoming[0].Header80)
	maxAlt2 := maxAlternateForkWork(peerBatches, forkPrev, longFirst)
	if longWork.Cmp(maxAlt2) <= 0 {
		t.Fatalf("long incoming %s should beat peer max %s", longWork, maxAlt2)
	}
}

func TestRejectIncomingForkIfPeerWorkHigher(t *testing.T) {
	var forkPrev, incFirst, altFirst [32]byte
	forkPrev[0] = 0xaa
	incFirst[0] = 0xbb
	altFirst[0] = 0xcc

	makeBatch := func(first [32]byte, bits uint32) []wire.DecodedHeader {
		h := make([]byte, 80)
		copy(h[4:36], forkPrev[:])
		h[76] = first[0]
		setHeaderBits(h, bits)
		return []wire.DecodedHeader{{Header80: h}}
	}

	incBatch := makeBatch(incFirst, testEasyBits)
	altBatch := makeBatch(altFirst, testHardBits)
	incWork, err := incomingChainWork(incBatch)
	if err != nil {
		t.Fatal(err)
	}
	if err := rejectIncomingForkIfPeerWorkHigher([][]wire.DecodedHeader{altBatch}, forkPrev, incFirst, incWork); err == nil {
		t.Fatal("expected reject when peer alternate has more work")
	}
	alt1Hash := pow.BlockHashLE(altBatch[0].Header80)
	alt2 := make([]byte, 80)
	copy(alt2, altBatch[0].Header80)
	copy(alt2[4:36], alt1Hash[:])
	alt2[76] ^= 0x33
	setHeaderBits(alt2, testHardBits)
	longIncoming := []wire.DecodedHeader{altBatch[0], {Header80: alt2}}
	longWork, err := incomingChainWork(longIncoming)
	if err != nil {
		t.Fatal(err)
	}
	longFirst := pow.BlockHashLE(longIncoming[0].Header80)
	if err := rejectIncomingForkIfPeerWorkHigher([][]wire.DecodedHeader{incBatch}, forkPrev, longFirst, longWork); err != nil {
		t.Fatalf("incoming should win: %v", err)
	}
}
