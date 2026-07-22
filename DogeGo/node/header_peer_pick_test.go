// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"sort"
	"testing"

	"dogego/chain"
	"dogego/wire"
)

func TestPickBlockPrimaryPeerPrefersArchival(t *testing.T) {
	probed := []headerSyncPeer{
		{addr: "header", dv: &wire.DecodedVersion{StartHeight: 5_000_000, Services: chain.ServiceNetworkLimited}},
		{addr: "tall-lim", dv: &wire.DecodedVersion{StartHeight: 4_999_000, Services: chain.ServiceNetworkLimited}},
		{addr: "full", dv: &wire.DecodedVersion{StartHeight: 3_000_000, Services: chain.ServiceNetwork}},
	}
	p, ok := pickBlockPrimaryPeer(probed, "header", 0)
	if !ok || p.addr != "full" {
		t.Fatalf("pick=%q ok=%v want full archival for genesis", p.addr, ok)
	}
}

func TestHeaderSyncProbeCandidatesFreshFirst(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NoteBlocksDelivered("old:22556", 100)
	discovered := []string{"seed-a:22556", "seed-b:22556", "seed-c:22556"}
	got := HeaderSyncProbeCandidates(discovered, s, []string{"manual:22556"})
	if len(got) < 4 {
		t.Fatalf("got %v", got)
	}
	if got[0] != "manual:22556" {
		t.Fatalf("expected addnode first, got %v", got[:3])
	}
	if got[1] != "seed-a:22556" {
		t.Fatalf("expected fresh DNS after addnode, got %v", got[:3])
	}
}

func TestHeaderSyncProbeCandidatesSpreadsIPv4Fresh(t *testing.T) {
	// Same /16 first - spread should interleave the other /16 before remaining same-group addrs.
	discovered := []string{
		"1.2.0.1:22556", "1.2.0.2:22556", "8.8.4.4:22556", "8.8.8.8:22556",
	}
	got := HeaderSyncProbeCandidates(discovered, nil, nil)
	if len(got) < 4 {
		t.Fatalf("got %v", got)
	}
	// First two entries should be one from 1.2/16 and one from 8.8/16 (order of groups follows first seen).
	if got[0] != "1.2.0.1:22556" {
		t.Fatalf("want 1.2.0.1 first, got %v", got[:4])
	}
	if got[1] != "8.8.4.4:22556" {
		t.Fatalf("want 8.8.4.4 second (spread), got %v", got[:4])
	}
}

func TestHeaderSyncPeerSortByStartHeight(t *testing.T) {
	peers := []headerSyncPeer{
		{addr: "a", dv: &wire.DecodedVersion{StartHeight: 100}},
		{addr: "b", dv: &wire.DecodedVersion{StartHeight: 500}},
		{addr: "c", dv: &wire.DecodedVersion{StartHeight: 200}},
	}
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].startHeight() > peers[j].startHeight()
	})
	if peers[0].addr != "b" || peers[1].addr != "c" || peers[2].addr != "a" {
		t.Fatalf("sort order: %s %s %s", peers[0].addr, peers[1].addr, peers[2].addr)
	}
}
