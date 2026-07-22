// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFeeHistorySaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fee_history.json")
	h := NewFeeHistory(10)
	h.Record([]uint64{50_000, 60_000})
	if err := h.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFeeHistoryFile(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.BlockCount() != 1 {
		t.Fatalf("blocks %d", loaded.BlockCount())
	}
	if got := loaded.EstimatePerKB(1); got != 60_000 {
		t.Fatalf("got %d", got)
	}
	_ = os.Remove(path)
}
