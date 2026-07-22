// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"testing"
)

func TestTxConfirmStatsUnconfRaisesConservativeEstimate(t *testing.T) {
	s := NewTxConfirmStats()
	s.SetBestSeenHeight(100)
	for i := 0; i < 15; i++ {
		s.TrackMempoolTx(fmt.Sprintf("%064x", i+1), 100, 50_000)
	}
	for i := 0; i < 12; i++ {
		s.RecordConfirm(6, 500_000)
		s.FlushBlock()
		s.AdvanceBlock(101 + int64(i))
	}
	eco := s.Estimate(6, false)
	con := s.Estimate(6, true)
	if con == 0 {
		t.Fatal("conservative")
	}
	if eco > 0 && con < eco {
		t.Fatalf("unconf should not lower conservative: con=%d eco=%d", con, eco)
	}
}

func TestTxConfirmStatsRemoveMempoolTx(t *testing.T) {
	s := NewTxConfirmStats()
	s.SetBestSeenHeight(10)
	s.TrackMempoolTx("abcd", 10, 120_000)
	if s.PendingMempoolTracks() != 1 {
		t.Fatalf("tracks %d", s.PendingMempoolTracks())
	}
	s.RemoveMempoolTx("abcd", 10)
	if s.PendingMempoolTracks() != 0 {
		t.Fatal("still tracked")
	}
}
