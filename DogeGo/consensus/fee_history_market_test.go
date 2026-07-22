// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestFeeHistoryBucketMarketStats(t *testing.T) {
	h := NewFeeHistory(10)
	h.Record([]uint64{100_000, 200_000})
	h.Record([]uint64{500_000})
	stats := h.BucketMarketStats()
	if stats["6"]["samples"].(int) != 2 {
		t.Fatalf("samples %#v", stats["6"])
	}
}

func TestFeeHistoryPersistBuckets(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/fee_history.json"
	h := NewFeeHistory(10)
	h.Record([]uint64{100_000, 300_000})
	if err := h.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFeeHistoryFile(path, 10)
	if err != nil || loaded == nil {
		t.Fatal(err)
	}
	stats := loaded.BucketMarketStats()
	if stats == nil || len(stats) == 0 {
		t.Fatal("expected bucket stats after load")
	}
}
