// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestNoteStubBlockHeavyPenalty(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NoteBlocksDelivered("stub:1", 5)
	before, ok := s.Stats("stub:1")
	if !ok || before.Score <= 0 {
		t.Fatalf("before=%+v", before)
	}
	s.NoteStubBlock("stub:1")
	after, ok := s.Stats("stub:1")
	if !ok {
		t.Fatal("missing stats")
	}
	if after.Score >= before.Score {
		t.Fatalf("score should drop after stub: before=%d after=%d", before.Score, after.Score)
	}
	if !after.InCooldown {
		t.Fatal("stub peer should be in cooldown")
	}
	if after.Failures != before.Failures+1 {
		t.Fatalf("failures=%d want %d", after.Failures, before.Failures+1)
	}
}

func TestPenalizeStubBlockPeer(t *testing.T) {
	s := NewBlockPeerScorer()
	penalizeStubBlockPeer(s, nil, "bad:1")
	st, ok := s.Stats("bad:1")
	if !ok || !st.InCooldown {
		t.Fatalf("stats=%+v ok=%v", st, ok)
	}
}
