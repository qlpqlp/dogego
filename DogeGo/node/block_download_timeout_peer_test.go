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

func TestMaybePenalizeDownloadTimeoutReleasesLane(t *testing.T) {
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 10
	bs.contiguousMu.Unlock()

	raw := &progressiveRawState{
		syncWorkers: 3,
		inFlight:     map[int64][32]byte{100: {}, 101: {}},
		inFlightLane: map[int64]int{100: 2, 101: 2},
		laneAddr:     map[int]string{2: "93.184.216.5:22556"},
		laneDownloadSince: map[int]time.Time{
			2: time.Now().Add(-10 * time.Minute),
		},
	}
	scorer := NewBlockPeerScorer()
	peer, timedOut := raw.maybePenalizeDownloadTimeout(bs, scorer, nil)
	if !timedOut || peer != "93.184.216.5:22556" {
		t.Fatalf("peer=%q timedOut=%v", peer, timedOut)
	}
	if len(raw.inFlight) != 0 {
		t.Fatalf("inFlight %v", raw.inFlight)
	}
}

func TestMaybePenalizeDownloadTimeoutWaits(t *testing.T) {
	raw := &progressiveRawState{
		syncWorkers: 2,
		inFlight:     map[int64][32]byte{5: {}},
		inFlightLane: map[int64]int{5: 1},
		laneDownloadSince: map[int]time.Time{
			1: time.Now().Add(-30 * time.Second),
		},
	}
	if _, timedOut := raw.maybePenalizeDownloadTimeout(&BlockStoreCtx{}, NewBlockPeerScorer(), nil); timedOut {
		t.Fatal("expected no timeout yet")
	}
	if len(raw.inFlight) != 1 {
		t.Fatal("claim should remain")
	}
}
