// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestShouldSeekThroughputBoostWhenAheadPipelineLarge(t *testing.T) {
	raw := &progressiveRawState{
		inFlight: make(map[int64][32]byte),
		contigRateSamples: []ibdRateSample{
			{at: time.Now().Add(-5 * time.Minute), cum: 1000},
			{at: time.Now(), cum: 1000},
		},
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderChain(dir, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, g80[:], 534_000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(5000)
	frontier := int64(5001)
	raw.inFlight[frontier] = [32]byte{}
	raw.stallingSince = time.Now().Add(-15 * time.Second)
	for h := frontier + 1; h <= frontier+400; h++ {
		raw.inFlight[h] = [32]byte{}
	}

	raw.mu.Lock()
	got := raw.shouldSeekThroughputBoostLocked(bs)
	raw.mu.Unlock()
	if !got {
		t.Fatal("expected boost when ahead pipeline is large and hole is blocked")
	}
}

func TestMaxFrontierClaimPeersDuringBoost(t *testing.T) {
	raw := &progressiveRawState{
		throughputBoostUntil: time.Now().Add(time.Minute),
	}
	raw.mu.Lock()
	got := raw.maxFrontierClaimPeersLocked(nil)
	raw.mu.Unlock()
	if got != ibdBoostMaxFrontierClaimPeers {
		t.Fatalf("boost max peers=%d want %d", got, ibdBoostMaxFrontierClaimPeers)
	}
}

func TestPeerBudgetHoldsDuringThroughputBoost(t *testing.T) {
	raw := &progressiveRawState{
		syncWorkers: 2,
		laneBudgetApplied: map[int]int{
			0: 128,
		},
		laneDelivery: map[int][]laneDeliverySample{
			0: {{at: time.Now(), n: 1}},
		},
		throughputBoostUntil: time.Now().Add(time.Minute),
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderChain(dir, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, g80[:], 534_000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(500_000)
	raw.mu.Lock()
	got := raw.peerInFlightBudgetLocked(bs, 0)
	raw.mu.Unlock()
	if got != ibdBoostLaneBudget {
		t.Fatalf("budget=%d want %d during boost (not deep-IBD floor 128)", got, ibdBoostLaneBudget)
	}
}

func TestTrimLaneAheadOfFrontier(t *testing.T) {
	raw := &progressiveRawState{
		inFlight:     map[int64][32]byte{100: {}, 150: {}, 200: {}, 250: {}},
		inFlightLane: map[int64]int{100: 0, 150: 0, 200: 0, 250: 0},
	}
	raw.mu.Lock()
	freed := raw.trimLaneAheadOfFrontierLocked(0, 100, 32)
	raw.mu.Unlock()
	if freed != 3 {
		t.Fatalf("freed=%d want 3 (heights above frontier+32 trimmed)", freed)
	}
	if _, ok := raw.inFlight[150]; ok {
		t.Fatal("expected far-ahead height 150 trimmed")
	}
}

func TestRecentShortContiguousBlocksPerMinute(t *testing.T) {
	now := time.Now()
	raw := &progressiveRawState{
		contigRateSamples: []ibdRateSample{
			{at: now.Add(-60 * time.Second), cum: 0},
			{at: now.Add(-30 * time.Second), cum: 1500},
			{at: now, cum: 3000},
		},
	}
	raw.mu.Lock()
	got := raw.recentShortContiguousBlocksPerMinuteLocked()
	raw.mu.Unlock()
	if got <= 0 {
		t.Fatalf("short rate=%v want >0", got)
	}
}
