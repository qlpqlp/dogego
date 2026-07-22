// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestEnrichIBDProgressSnapshotConnectCatchUpTuning(t *testing.T) {
	bs := testBodyIBDBlockStoreCtx(t, 534_000, 20_000, 5000)
	snap := map[string]interface{}{}
	enrichIBDProgressSnapshot(snap, bs.Journal, bs)
	if snap["connect_lag"] != int64(15_000) {
		t.Fatalf("connect_lag=%v want 15000", snap["connect_lag"])
	}
	if snap["connect_catch_up_passes"] != 8 {
		t.Fatalf("passes=%v want 8", snap["connect_catch_up_passes"])
	}
	if snap["connect_catch_up_batch"] != 128 {
		t.Fatalf("batch=%v want 128", snap["connect_catch_up_batch"])
	}
	if snap["connect_catch_up_interval_ms"] != int64(500) {
		t.Fatalf("interval_ms=%v want 500", snap["connect_catch_up_interval_ms"])
	}
}

func TestRawBlocksAheadOfContiguous(t *testing.T) {
	if got := rawBlocksAheadOfContiguous(9977, 10006); got != 28 {
		t.Fatalf("got %d want 28", got)
	}
	if got := rawBlocksAheadOfContiguous(10005, 10006); got != 0 {
		t.Fatalf("frontier gap got %d want 0", got)
	}
	if got := rawBlocksAheadOfContiguous(1, 2); got != 0 {
		t.Fatalf("single missing got %d want 0", got)
	}
	if got := rawBlocksAheadOfContiguous(1, 3); got != 1 {
		t.Fatalf("hole at 2 got %d want 1", got)
	}
}

func TestBodyIBDEtaMinutes(t *testing.T) {
	snap := map[string]interface{}{"blocks_per_minute": 12.0}
	bodyIBDEtaMinutes(snap, 534_000, 11_712)
	got, ok := snap["body_ibd_eta_minutes"].(int64)
	if !ok || got != 43_524 {
		t.Fatalf("eta=%v want 43524", snap["body_ibd_eta_minutes"])
	}
	snap2 := map[string]interface{}{}
	bodyIBDEtaMinutes(snap2, 534_000, 11_712)
	if _, ok := snap2["body_ibd_eta_minutes"]; ok {
		t.Fatal("expected no eta without download rate")
	}
}
