// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"path/filepath"
	"testing"
)

func TestConfirmStatsUnconfPersistRoundtrip(t *testing.T) {
	h := NewFeeHistory(8)
	h.confirmStats.SetBestSeenHeight(10)
	h.confirmStats.TrackMempoolTx("aa", 10, 200_000)

	dir := t.TempDir()
	path := filepath.Join(dir, "fee_history.json")
	if err := h.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFeeHistoryFile(path, 8)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ConfirmStatsPendingTracks() != 1 {
		t.Fatalf("tracks %d", loaded.ConfirmStatsPendingTracks())
	}
	var unconfTotal float64
	for _, row := range loaded.confirmStats.unconfRing {
		for _, v := range row {
			unconfTotal += v
		}
	}
	if unconfTotal < 1 {
		t.Fatal("expected unconf ring count")
	}
	loaded.CatchUpBlockHeights(12)
	if loaded.confirmStats.bestSeenHeight != 12 {
		t.Fatalf("tip %d", loaded.confirmStats.bestSeenHeight)
	}
}

func TestCatchUpBlockHeightsRollsUnconf(t *testing.T) {
	h := NewFeeHistory(8)
	h.confirmStats.SetBestSeenHeight(5)
	h.confirmStats.TrackMempoolTx("bb", 5, 100_000)
	h.CatchUpBlockHeights(7)
	if h.confirmStats.bestSeenHeight != 7 {
		t.Fatalf("height %d", h.confirmStats.bestSeenHeight)
	}
}

func TestCatchUpBlockHeightsLargeGapJumps(t *testing.T) {
	h := NewFeeHistory(8)
	h.confirmStats.SetBestSeenHeight(5)
	h.CatchUpBlockHeights(6_000_000)
	if h.confirmStats.bestSeenHeight != 6_000_000 {
		t.Fatalf("height %d", h.confirmStats.bestSeenHeight)
	}
}
