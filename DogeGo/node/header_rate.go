// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"sync"
	"time"
)

type headerRateSample struct {
	at  time.Time
	cum int64
}

var headerRate struct {
	mu      sync.Mutex
	total   int64
	started time.Time
	samples []headerRateSample
}

func recordHeadersAppended(n int) {
	if n <= 0 {
		return
	}
	now := time.Now()
	headerRate.mu.Lock()
	defer headerRate.mu.Unlock()
	if headerRate.started.IsZero() {
		headerRate.started = now
	}
	headerRate.total += int64(n)
	headerRate.samples = append(headerRate.samples, headerRateSample{at: now, cum: headerRate.total})
	cutoff := now.Add(-ibdRateWindow)
	i := 0
	for i < len(headerRate.samples) && headerRate.samples[i].at.Before(cutoff) {
		i++
	}
	if i > 0 {
		headerRate.samples = append([]headerRateSample(nil), headerRate.samples[i:]...)
	}
}

// RecentHeadersPerMinute is validated-header throughput over the last few minutes.
func RecentHeadersPerMinute() float64 {
	headerRate.mu.Lock()
	defer headerRate.mu.Unlock()
	if len(headerRate.samples) < 2 {
		return 0
	}
	first, last := headerRate.samples[0], headerRate.samples[len(headerRate.samples)-1]
	elapsed := last.at.Sub(first.at)
	if elapsed < ibdRateMinWindow {
		return 0
	}
	delta := last.cum - first.cum
	if delta <= 0 {
		return 0
	}
	return float64(delta) / elapsed.Minutes()
}
