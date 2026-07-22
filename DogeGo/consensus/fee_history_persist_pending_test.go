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

func TestFeeHistoryPersistPendingMempool(t *testing.T) {
	h := NewFeeHistory(8)
	h.TrackMempoolAdmission("tx1", 120_000, 50)
	h.confirmStats.SetBestSeenHeight(50)

	dir := t.TempDir()
	path := filepath.Join(dir, "fee_history.json")
	if err := h.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFeeHistoryFile(path, 8)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PendingMempoolFeeTracks() != 1 {
		t.Fatalf("pending %d", loaded.PendingMempoolFeeTracks())
	}
	if n := loaded.ApplyLoadedPendingTracks(50); n != 1 {
		t.Fatalf("applied %d want 1", n)
	}
	if loaded.ConfirmStatsPendingTracks() != 1 {
		t.Fatalf("confirm pending %d", loaded.ConfirmStatsPendingTracks())
	}
}

func TestApplyLoadedPendingTracksDropsStale(t *testing.T) {
	h := NewFeeHistory(8)
	h.TrackMempoolAdmission("old", 50_000, 1)
	if n := h.ApplyLoadedPendingTracks(100); n != 0 {
		t.Fatalf("applied stale %d", n)
	}
	if h.PendingMempoolFeeTracks() != 0 {
		t.Fatal("stale pending should be dropped")
	}
}
