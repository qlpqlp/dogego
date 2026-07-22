// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestScanClaimRangeRespectsPerLaneInFlightCap(t *testing.T) {
	s := &progressiveRawState{
		inFlight:     map[int64][32]byte{},
		inFlightLane: map[int64]int{},
	}
	for h := int64(0); h < 16; h++ {
		s.inFlight[h] = [32]byte{}
		s.inFlightLane[h] = 0
	}
	inFlightSnap := make(map[int64][32]byte, len(s.inFlight))
	for h, hash := range s.inFlight {
		inFlightSnap[h] = hash
	}
	claim := s.planClaimRange(&BlockStoreCtx{}, nil, nil, 0, 0, 100, 100, 0, 0, 1, 16, inFlightSnap)
	if len(claim.heights) != 0 {
		t.Fatalf("expected no new claims when lane at cap, got %d", len(claim.heights))
	}
}

func TestPenalizeBlockPeerUpdatesAddrbook(t *testing.T) {
	book := NewAddrBook()
	book.AddSeen("93.184.216.10:22556")
	penalizeBlockPeer(nil, book, "93.184.216.10:22556", true)
	book.mu.Lock()
	rec := book.by["93.184.216.10:22556"]
	book.mu.Unlock()
	if rec == nil || rec.Attempts < 1 {
		t.Fatalf("addrbook attempts %v", rec)
	}
}
