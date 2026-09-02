// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"testing"
	"time"
)

func TestAssistAddrInCooldownAfterBadMagic(t *testing.T) {
	scorer := NewBlockPeerScorer()
	addr := "35.73.87.31:22556"
	penalizeWrongNetworkPeer(scorer, nil, addr, errors.New("bad magic a0860100, expected c0c0c0c0"))
	if !assistAddrInCooldown(scorer, addr) {
		t.Fatal("bad-magic peer must be in cooldown")
	}
	st, ok := scorer.Stats(addr)
	if !ok || !st.InCooldown {
		t.Fatalf("stats %#v ok=%v", st, ok)
	}
	// Quarantine must outlast a normal hard reject window enough that assist cannot spin.
	scorer.mu.Lock()
	until := scorer.entries[addr].cooldownTo
	scorer.mu.Unlock()
	if time.Until(until) < time.Hour {
		t.Fatalf("wrong-net cooldown too short: remaining %v", time.Until(until))
	}
}

func TestAssistAddrInCooldownNilSafe(t *testing.T) {
	if assistAddrInCooldown(nil, "1.2.3.4:22556") {
		t.Fatal("nil scorer")
	}
	if assistAddrInCooldown(NewBlockPeerScorer(), "") {
		t.Fatal("empty addr")
	}
}
