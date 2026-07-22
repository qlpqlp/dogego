// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"testing"
	"time"
)

func TestRollingMinFeeUsesConfiguredIncremental(t *testing.T) {
	p := New(100)
	p.SetIncrementalRelayFeePerKB(300_000)
	p.mu.Lock()
	p.rollingMinFeePerKB = 400_000
	p.lastRollingFeeUpdate = time.Now().Unix()
	p.blockSinceLastRollingFeeBump = false
	p.mu.Unlock()
	got := p.MinRelayFeePerKB()
	if got < 300_000 {
		t.Fatalf("min relay %d want >= 300000", got)
	}
}
