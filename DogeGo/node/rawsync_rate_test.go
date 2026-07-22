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

func TestRecentBlocksPerMinutePrefersWindow(t *testing.T) {
	s := &progressiveRawState{}
	now := time.Now()
	s.ibdStarted = now.Add(-2 * time.Hour)
	s.blocksStoredIBD = 1000
	s.rateSamples = []ibdRateSample{
		{at: now.Add(-8 * time.Minute), cum: 800},
		{at: now.Add(-1 * time.Minute), cum: 1000},
	}
	s.mu.Lock()
	recent := s.recentBlocksPerMinuteLocked()
	s.mu.Unlock()
	// 200 blocks in 7 minutes ≈ 28.57 blk/min
	if recent < 25 || recent > 35 {
		t.Fatalf("recent bpm=%v want ~28.6", recent)
	}
	snap := s.snapshot()
	bpm, _ := snap["blocks_per_minute"].(float64)
	if bpm < 25 || bpm > 35 {
		t.Fatalf("snapshot blocks_per_minute=%v want recent window", bpm)
	}
	life, _ := snap["blocks_per_minute_lifetime"].(float64)
	if life <= 0 || life > 20 {
		t.Fatalf("lifetime bpm=%v want diluted ~8", life)
	}
}
