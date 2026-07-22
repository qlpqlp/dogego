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

func TestRollingMinFeeAfterEviction(t *testing.T) {
	p := New(2)
	p.mu.Lock()
	p.rollingMinFeePerKB = 0
	p.lastRollingFeeUpdate = time.Now().Unix()
	p.blockSinceLastRollingFeeBump = false
	p.mu.Unlock()

	p.trackPackageRemoved(500_000, 500)
	rate := p.MinRelayFeePerKB()
	if rate < defaultIncrementalRelayFeePerKB {
		t.Fatalf("min rate %d want at least %d", rate, defaultIncrementalRelayFeePerKB)
	}
}

func TestNoteBlockFoundResetsDecayGate(t *testing.T) {
	p := New(10)
	p.mu.Lock()
	p.rollingMinFeePerKB = 500_000
	p.blockSinceLastRollingFeeBump = false
	p.mu.Unlock()
	p.NoteBlockFound()
	p.mu.Lock()
	if !p.blockSinceLastRollingFeeBump {
		t.Fatal("expected block bump flag")
	}
	got := p.rollingMinFeePerKB
	p.mu.Unlock()
	if p.MinRelayFeePerKB() != uint64(got) {
		t.Fatal("expected undecayed rate while blockSinceLastRollingFeeBump")
	}
}
