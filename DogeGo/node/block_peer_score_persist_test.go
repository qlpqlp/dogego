// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
)

func TestBlockPeerScoresPersistServices(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "block_peer_scores.json")
	s := NewBlockPeerScorer()
	s.NotePeerHandshake("93.184.216.10:22556", chain.ServiceNetwork, 5_000_000)
	s.NoteBlocksDelivered("93.184.216.10:22556", 3)
	if err := s.SaveToFile(path); err != nil {
		t.Fatal(err)
	}
	s2 := NewBlockPeerScorer()
	if err := s2.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	svc, start, ok := s2.peerMeta("93.184.216.10:22556")
	if !ok || svc != chain.ServiceNetwork || start != 5_000_000 {
		t.Fatalf("meta svc=%x start=%d ok=%v", svc, start, ok)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestMergeDiscoveryCandidatesArchival(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NotePeerHandshake("lim:1", chain.ServiceNetworkLimited, 5_000_000)
	s.NotePeerHandshake("full:1", chain.ServiceNetwork, 3_000_000)
	got := s.MergeDiscoveryCandidates([]string{"lim:1", "full:1"}, 50)
	if len(got) < 2 || got[0] != "full:1" {
		t.Fatalf("merge=%v want full first for height 50", got)
	}
}
