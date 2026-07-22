// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestFeeHistoryHorizonMax(t *testing.T) {
	h := NewFeeHistory(10)
	h.Record([]uint64{10_000})
	h.Record([]uint64{500_000})
	if got := h.EstimatePerKB(1); got != 500_000 {
		t.Fatalf("depth 1: got %d", got)
	}
	if got := h.EstimatePerKBHorizonMax(6); got != 500_000 {
		t.Fatalf("horizon max: got %d want 500k", got)
	}
}
