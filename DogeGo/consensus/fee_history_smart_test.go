// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEstimateConfirmStatsSmartWalk(t *testing.T) {
	h := NewFeeHistory(10)
	for i := 0; i < 15; i++ {
		h.RecordMempoolConfirmedSamples([]MempoolConfirmSample{
			{FeeratePerKB: 300_000, BlocksWaited: 8},
		})
	}
	r, ab := h.EstimateConfirmStatsSmart(6, true)
	if r == 0 {
		t.Fatal("expected estimate")
	}
	if ab < 6 || ab > maxConfirmStatsConfirms {
		t.Fatalf("answer blocks %d", ab)
	}
}

func TestEstimateConfirmStatsSmartLongHorizon(t *testing.T) {
	h := NewFeeHistory(10)
	for i := 0; i < 12; i++ {
		h.RecordMempoolConfirmedSamples([]MempoolConfirmSample{
			{FeeratePerKB: 200_000, BlocksWaited: 3},
		})
	}
	r, ab := h.EstimateConfirmStatsSmart(48, true)
	if r == 0 {
		t.Fatal("expected long-horizon capped estimate")
	}
	if ab != ClosestStandardBucketTarget(48) {
		t.Fatalf("answer %d want %d", ab, ClosestStandardBucketTarget(48))
	}
}
