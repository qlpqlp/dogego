// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestInflightHeightsForLane(t *testing.T) {
	s := &progressiveRawState{
		inFlight:     map[int64][32]byte{10: {}, 11: {}},
		inFlightLane: map[int64]int{10: 0, 11: 2},
	}
	h0 := s.InflightHeightsForLane(0)
	if len(h0) != 1 || h0[0] != 10 {
		t.Fatalf("lane 0: %v", h0)
	}
	h2 := s.InflightHeightsForLane(2)
	if len(h2) != 1 || h2[0] != 11 {
		t.Fatalf("lane 2: %v", h2)
	}
}

func TestReleaseLaneInFlight(t *testing.T) {
	s := &progressiveRawState{
		inFlight:     map[int64][32]byte{10: {}, 11: {}, 12: {}},
		inFlightLane: map[int64]int{10: 0, 11: 0, 12: 2},
	}
	if n := s.ReleaseLaneInFlight(0); n != 2 {
		t.Fatalf("freed %d want 2", n)
	}
	if len(s.inFlight) != 1 || s.inFlightLane[12] != 2 {
		t.Fatalf("other lane kept: inFlight=%v lanes=%v", s.inFlight, s.inFlightLane)
	}
}

func TestSyncLaneForPeer(t *testing.T) {
	raw := &progressiveRawState{syncWorkers: 4}
	assist := []AssistPeerSnapshot{{Addr: "9.9.9.9:22556", Lane: 2}}
	if syncLaneForPeer("1.2.3.4:22556", "1.2.3.4:22556", assist, raw) != 0 {
		t.Fatal("primary lane")
	}
	if syncLaneForPeer("9.9.9.9:22556", "1.2.3.4:22556", assist, raw) != 2 {
		t.Fatal("assist lane")
	}
}
