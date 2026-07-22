// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestPrimaryExclude(t *testing.T) {
	var ex PrimaryExclude
	if ex.Addr() != "" {
		t.Fatal("expected empty")
	}
	ex.Set("peer:1")
	if ex.Addr() != "peer:1" {
		t.Fatalf("addr %q", ex.Addr())
	}
	ex.Set("peer:2")
	if ex.Addr() != "peer:2" {
		t.Fatalf("addr %q", ex.Addr())
	}
}

func TestBlockAssistRespectsDynamicPrimaryExclude(t *testing.T) {
	s := NewBlockPeerScorer()
	all := []string{"a:1", "b:2", "c:3", "d:4"}
	var ex PrimaryExclude
	ex.Set("a:1")
	w0 := s.CandidatesForWorker(all, ex.Addr(), 0, 2, -1)
	ex.Set("b:2")
	w1 := s.CandidatesForWorker(all, ex.Addr(), 1, 2, -1)
	for _, a := range w0 {
		if a == "a:1" {
			t.Fatalf("worker0 dialed primary: %v", w0)
		}
	}
	for _, a := range w1 {
		if a == "b:2" {
			t.Fatalf("worker1 dialed primary: %v", w1)
		}
	}
}
