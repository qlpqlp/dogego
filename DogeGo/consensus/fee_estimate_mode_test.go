// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEstimateFeeFromRatesMax(t *testing.T) {
	if got := EstimateFeeFromRatesMax([]uint64{10, 50, 30}); got != 50 {
		t.Fatalf("max %d", got)
	}
}

func TestClosestStandardBucketTarget(t *testing.T) {
	if got := ClosestStandardBucketTarget(5); got != 6 {
		t.Fatalf("got %d want 6", got)
	}
	if got := ClosestStandardBucketTarget(100); got != 144 {
		t.Fatalf("got %d want 144", got)
	}
}

func TestFeeHistoryConservative(t *testing.T) {
	h := NewFeeHistory(10)
	h.Record([]uint64{10_000})
	h.Record([]uint64{100_000, 200_000})
	if got := h.EstimatePerKB(6); got < 100_000 {
		t.Fatalf("economical %d", got)
	}
	if got := h.EstimatePerKBConservative(6); got < 200_000 {
		t.Fatalf("conservative %d", got)
	}
}
