// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
	"time"
)

func TestMaybePenalizeStallingPeerReleasesFrontier(t *testing.T) {
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 5
	bs.contiguousMu.Unlock()

	raw := &progressiveRawState{
		inFlight:     map[int64][32]byte{6: {}},
		inFlightLane: map[int64]int{6: 0},
		laneAddr:     map[int]string{0: "93.184.216.1:22556"},
		stallingSince: time.Now().Add(-3 * time.Second),
	}
	scorer := NewBlockPeerScorer()
	stallPeer, stalled := raw.maybePenalizeStallingPeer(bs, scorer, nil)
	if !stalled || stallPeer == "" {
		t.Fatalf("expected stall penalty, got peer=%q stalled=%v", stallPeer, stalled)
	}
	if len(raw.inFlight) != 0 {
		t.Fatalf("inFlight %v want empty", raw.inFlight)
	}
	stats, ok := scorer.Stats("93.184.216.1:22556")
	if !ok || stats.Failures == 0 {
		t.Fatalf("peer failure not recorded: %+v ok=%v", stats, ok)
	}
}

func TestMaybePenalizeStallingPeerWaitsForTimeout(t *testing.T) {
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 0
	bs.contiguousMu.Unlock()

	raw := &progressiveRawState{
		inFlight:     map[int64][32]byte{1: {}},
		inFlightLane: map[int64]int{1: 0},
		stallingSince: time.Now().Add(-500 * time.Millisecond),
	}
	if _, stalled := raw.maybePenalizeStallingPeer(bs, NewBlockPeerScorer(), nil); stalled {
		t.Fatal("expected no penalty before blockStallingTimeout")
	}
	if len(raw.inFlight) != 1 {
		t.Fatal("claim should remain in flight")
	}
}
