// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"sync"
	"testing"
)

func TestCandidatesForWorker(t *testing.T) {
	all := []string{"a:1", "b:1", "c:1", "d:1"}
	scorer := NewBlockPeerScorer()
	w0 := scorer.CandidatesForWorker(all, "x", 0, 2, -1)
	w1 := scorer.CandidatesForWorker(all, "x", 1, 2, -1)
	if len(w0) != 2 || len(w1) != 2 {
		t.Fatalf("w0=%v w1=%v", w0, w1)
	}
	if w0[0] != "a:1" || w0[1] != "c:1" || w1[0] != "b:1" || w1[1] != "d:1" {
		t.Fatalf("partition mismatch w0=%v w1=%v", w0, w1)
	}
	ex := scorer.CandidatesForWorker(all, "b:1", 1, 2, -1)
	if len(ex) != 1 || ex[0] != "c:1" {
		t.Fatalf("exclude: %v", ex)
	}
}

func TestProgressiveRawStateMutex(t *testing.T) {
	var s progressiveRawState
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.OnTipChanged(int64(i))
			_ = s.useShortReadDeadline()
		}()
	}
	wg.Wait()
}
