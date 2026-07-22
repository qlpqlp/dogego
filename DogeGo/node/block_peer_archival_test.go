// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
)

func TestDialableOrderForBlockPrefersFullNetwork(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NotePeerHandshake("full:1", chain.ServiceNetwork, 5_000_000)
	s.NotePeerHandshake("lim:1", chain.ServiceNetworkLimited, 5_000_000)
	ordered := s.DialableOrderForBlock([]string{"lim:1", "full:1"}, "", 100)
	if len(ordered) != 2 || ordered[0] != "full:1" {
		t.Fatalf("order=%v want full first for height 100", ordered)
	}
}

func TestDialableOrderForBlockLimitedOKNearTip(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NotePeerHandshake("lim:1", chain.ServiceNetworkLimited, 10_000)
	ordered := s.DialableOrderForBlock([]string{"lim:1", "unknown:1"}, "", 9900)
	if len(ordered) != 2 || ordered[0] != "lim:1" {
		t.Fatalf("limited peer near tip should rank first: %v", ordered)
	}
}

func TestCandidatesForWorkerArchival(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NotePeerHandshake("a:1", chain.ServiceNetworkLimited, 1000)
	s.NotePeerHandshake("b:1", chain.ServiceNetwork, 1000)
	all := []string{"a:1", "b:1"}
	w0 := s.CandidatesForWorker(all, "", 0, 1, 50)
	if len(w0) != 2 || w0[0] != "b:1" {
		t.Fatalf("worker order=%v want archival b:1 first", w0)
	}
}

func TestDialableOrderForBlockUnknownDeprioritizedAncient(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NotePeerHandshake("full:1", chain.ServiceNetwork, 5_000_000)
	ordered := s.DialableOrderForBlock([]string{"unknown:1", "full:1"}, "", 10_006)
	if len(ordered) != 2 || ordered[0] != "full:1" {
		t.Fatalf("order=%v want known full NODE_NETWORK first for height 10006", ordered)
	}
}

func TestDialableOrderForBlockRejectsLimitedAncient(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NotePeerHandshake("lim:1", chain.ServiceNetwork|chain.ServiceNetworkLimited, 6_233_574)
	s.NotePeerHandshake("full:1", chain.ServiceNetwork, 5_000_000)
	ordered := s.DialableOrderForBlock([]string{"lim:1", "full:1"}, "", 10_006)
	if len(ordered) != 2 || ordered[0] != "full:1" {
		t.Fatalf("order=%v want archival full peer first for height 10006", ordered)
	}
}
