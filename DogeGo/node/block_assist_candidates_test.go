// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"

	"dogego/chain"
)

func TestBlockAssistCandidatesRefresh(t *testing.T) {
	s := NewBlockPeerScorer()
	good := "1.2.3.4:22556"
	seed1 := "8.8.8.8:22556"
	seed2 := "8.8.4.4:22556"
	seed3 := "8.8.4.5:22556"
	learned := "9.9.9.9:22556"
	add := "1.2.3.5:22556"
	s.NoteBlocksDelivered(good, 5)
	c := NewBlockAssistCandidates([]string{seed1, seed2}, s)
	if c.Len() != 3 { // good + seeds
		t.Fatalf("initial len %d", c.Len())
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	pm.NoteAddr(learned)
	c.Refresh([]string{seed3}, pm, s, []string{add})
	snap := c.Snapshot()
	seen := make(map[string]bool)
	for _, a := range snap {
		seen[a] = true
	}
	if !seen[good] || !seen[learned] || !seen[seed3] {
		t.Fatalf("refresh missing entries: %v", snap)
	}
}

func TestBlockAssistCandidatesSpreadsDiscovery(t *testing.T) {
	c := NewBlockAssistCandidates([]string{
		"1.2.0.1:22556", "1.2.0.2:22556", "8.8.4.4:22556",
	}, nil)
	snap := c.Snapshot()
	if len(snap) < 3 {
		t.Fatalf("snap %v", snap)
	}
	// addnode-free pool: spread interleaves 8.8 before remaining 1.2.x
	if snap[0] != "1.2.0.1:22556" || snap[1] != "8.8.4.4:22556" {
		t.Fatalf("spread order %v", snap)
	}
}

func TestBlockAssistCandidatesAddnodeFirst(t *testing.T) {
	c := NewBlockAssistCandidates([]string{"1.2.0.1:22556", "8.8.8.8:22556"}, nil)
	c.Refresh([]string{"1.2.0.2:22556"}, nil, nil, []string{"addnode.example:22556"})
	snap := c.Snapshot()
	if len(snap) == 0 || snap[0] != "addnode.example:22556" {
		t.Fatalf("addnode first, got %v", snap)
	}
}

func mustTestnetParams(t *testing.T) chain.Params {
	t.Helper()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
