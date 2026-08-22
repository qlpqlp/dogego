// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
	"time"
)

func TestEffectiveLaneDownloadTimeoutProgressing(t *testing.T) {
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 5000
	bs.contiguousMu.Unlock()

	s := &progressiveRawState{}
	s.mu.Lock()
	stalled := s.effectiveLaneDownloadTimeoutLocked(bs, 6, 0)
	s.mu.Unlock()
	if stalled != bodyIBDBlockDownloadTimeout {
		t.Fatalf("no progress timeout=%v want %v", stalled, bodyIBDBlockDownloadTimeout)
	}

	now := time.Now()
	s.laneDelivery = map[int][]laneDeliverySample{
		0: {{at: now.Add(-5 * time.Second), n: 10}},
	}
	s.mu.Lock()
	progressing := s.effectiveLaneDownloadTimeoutLocked(bs, 6, 0)
	s.mu.Unlock()
	if progressing != bodyIBDProgressDownloadTimeout {
		t.Fatalf("progressing timeout=%v want %v", progressing, bodyIBDProgressDownloadTimeout)
	}
}

func TestBlockPeerScorerUsesDeliveryEWMA(t *testing.T) {
	s := NewBlockPeerScorer()
	s.NoteBlocksDelivered("fast:1", 20)
	st, ok := s.Stats("fast:1")
	if !ok || st.Blocks != 20 {
		t.Fatalf("stats %#v ok=%v", st, ok)
	}
	if st.Score <= blockPeerScoreSuccessWeight {
		t.Fatalf("EWMA score=%d expected >> legacy per-block weight", st.Score)
	}
	s.NoteBlocksDelivered("slow:1", 1)
	stSlow, _ := s.Stats("slow:1")
	if stSlow.Score >= st.Score {
		t.Fatalf("slow score %d should rank below fast %d", stSlow.Score, st.Score)
	}
}
