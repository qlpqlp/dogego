// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestFeeHistoryEstimatePerKBEconomical(t *testing.T) {
	h := NewFeeHistory(10)
	h.Record([]uint64{500_000, 600_000})
	h.Record([]uint64{50_000, 60_000})
	if got := h.EstimatePerKBEconomical(6); got == 0 {
		t.Fatal("expected economical estimate")
	}
	if got := h.EstimatePerKBConservative(6); got < h.EstimatePerKBEconomical(6) {
		t.Fatalf("conservative %d < economical %d", got, h.EstimatePerKBEconomical(6))
	}
}

func TestEstimatePendingMempoolMinPerKB(t *testing.T) {
	h := NewFeeHistory(10)
	h.TrackMempoolAdmission("aa", 200_000, 1)
	h.TrackMempoolAdmission("bb", 80_000, 1)
	if got := h.EstimatePendingMempoolMinPerKB(); got != 80_000 {
		t.Fatalf("min pending %d", got)
	}
}
