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

func TestConfirmStatsCurBatchPersistRoundtrip(t *testing.T) {
	h := NewFeeHistory(8)
	h.confirmStats.SetBestSeenHeight(10)
	h.confirmStats.RecordConfirm(2, 180_000)
	// cur batch not flushed yet
	if !h.confirmStats.hasCurBatch() {
		t.Fatal("expected cur batch before flush")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "fee_history.json")
	if err := h.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFeeHistoryFile(path, 8)
	if err != nil {
		t.Fatal(err)
	}
	bi := feerateBucketIndex(loaded.confirmStats.buckets, 180_000)
	if loaded.confirmStats.curConf[1][bi] < 1 {
		t.Fatal("cur_conf not restored")
	}
}
