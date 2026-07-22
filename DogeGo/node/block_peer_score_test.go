// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestBlockPeerScorerStatsAndTop(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NoteBlocksDelivered("a:1", 3)
	s.NoteDialFailure("b:2")
	if _, ok := s.Stats("missing"); ok {
		t.Fatal("unknown addr")
	}
	st, ok := s.Stats("a:1")
	if !ok || st.Score <= 0 || st.Blocks != 3 {
		t.Fatalf("a:1 stats %#v ok=%v", st, ok)
	}
	tops := s.TopPeers(2)
	if len(tops) < 1 || tops[0].Addr != "a:1" {
		t.Fatalf("tops %#v", tops)
	}
}

func TestMergeDiscoveryCandidatesSpreadsFresh(t *testing.T) {
	s := NewBlockPeerScorer()
	discovered := []string{"1.2.0.1:22556", "1.2.0.2:22556", "8.8.4.4:22556"}
	got := s.MergeDiscoveryCandidates(discovered, -1)
	if len(got) < 3 {
		t.Fatalf("got %v", got)
	}
	// Fresh tail is spread: first two entries should be one 1.2 and one 8.8 when no known peers.
	if got[0] != "1.2.0.1:22556" || got[1] != "8.8.4.4:22556" {
		t.Fatalf("spread order %v", got)
	}
}

func TestBlockPeerScorerDialableOrderPrefersReady(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NoteDialFailure("bad:1")
	s.NoteBlocksDelivered("good:1", 5)
	got := s.DialableOrder([]string{"bad:1", "good:1", "fresh:1"}, "")
	if len(got) != 3 || got[0] != "good:1" || got[1] != "fresh:1" || got[2] != "bad:1" {
		t.Fatalf("dialable order %v", got)
	}
	st, _ := s.Stats("bad:1")
	if !st.InCooldown {
		t.Fatal("bad:1 expected cooldown")
	}
	// All cooling: still returns peers so startup can retry.
	s.NoteDialFailure("x:1")
	s.NoteDialFailure("y:1")
	allCD := s.DialableOrder([]string{"x:1", "y:1"}, "")
	if len(allCD) != 2 {
		t.Fatalf("all cooldown fallback %v", allCD)
	}
}

func TestOrderCandidatesDoesNotCreateEntries(t *testing.T) {
	s := NewBlockPeerScorer()
	_ = s.OrderCandidates([]string{"never-dialed:22556"}, "")
	if _, ok := s.Stats("never-dialed:22556"); ok {
		t.Fatal("OrderCandidates must not add scorer entries for unseen addresses")
	}
}
