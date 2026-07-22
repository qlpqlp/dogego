// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"testing"

	"dogego/pow"
	"dogego/wire"
)

// TestMaxAlternateForkWorkRejectsWeakerIncoming documents Core-shaped fork election: a peer
// branch with more cumulative work on the same fork parent wins over a weaker incoming batch.
func TestMaxAlternateForkWorkRejectsWeakerIncoming(t *testing.T) {
	var forkPrev [32]byte
	forkPrev[0] = 0xaa

	makeHeader := func(prev [32]byte, nonce byte, bits uint32) []byte {
		h := make([]byte, 80)
		copy(h[4:36], prev[:])
		h[76] = nonce
		binary.LittleEndian.PutUint32(h[72:76], bits)
		return h
	}

	// Incoming fork: one block, easy bits (low work).
	inEasy := makeHeader(forkPrev, 1, 0x1f00ffff)
	incoming := []wire.DecodedHeader{{Header80: inEasy}}
	inWork, err := incomingChainWork(incoming)
	if err != nil {
		t.Fatal(err)
	}

	// Peer fork: two blocks, harder bits (more total work).
	hard1 := makeHeader(forkPrev, 2, 0x1c00ffff)
	id1 := pow.BlockHashLE(hard1)
	hard2 := makeHeader(id1, 3, 0x1c00ffff)
	peerBatch := []wire.DecodedHeader{
		{Header80: hard1},
		{Header80: hard2},
	}
	incomingFirst := pow.BlockHashLE(inEasy)
	maxAlt := maxAlternateForkWork([][]wire.DecodedHeader{peerBatch}, forkPrev, incomingFirst)
	if maxAlt.Cmp(inWork) <= 0 {
		t.Fatalf("peer work %s not greater than incoming %s", maxAlt, inWork)
	}
}

// TestMaxAlternateForkWorkIgnoresSameFirstHash ensures identical fork tips are not treated as alternates.
func TestMaxAlternateForkWorkIgnoresSameFirstHash(t *testing.T) {
	var forkPrev [32]byte
	forkPrev[1] = 0xbb
	h := make([]byte, 80)
	copy(h[4:36], forkPrev[:])
	h[76] = 9
	binary.LittleEndian.PutUint32(h[72:76], 0x1e0ffff0)
	decoded := []wire.DecodedHeader{{Header80: h}}
	first := pow.BlockHashLE(h)
	maxAlt := maxAlternateForkWork([][]wire.DecodedHeader{decoded}, forkPrev, first)
	if maxAlt.Sign() != 0 {
		t.Fatalf("expected zero alternate work, got %s", maxAlt)
	}
}
