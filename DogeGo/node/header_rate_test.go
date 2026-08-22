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

func TestRecentHeadersPerMinute(t *testing.T) {
	headerRate.mu.Lock()
	headerRate.total = 0
	headerRate.started = time.Time{}
	now := time.Now()
	headerRate.samples = []headerRateSample{
		{at: now.Add(-10 * time.Second), cum: 0},
		{at: now, cum: 2000},
	}
	headerRate.mu.Unlock()
	got := RecentHeadersPerMinute()
	if got < 10000 || got > 14000 {
		t.Fatalf("headers/min=%v want ~12000", got)
	}
}
