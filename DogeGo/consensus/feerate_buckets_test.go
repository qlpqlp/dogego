// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestDefaultFeerateBucketSpacing(t *testing.T) {
	b := DefaultFeerateBucketUpperBounds()
	if len(b) < 10 {
		t.Fatalf("buckets %d", len(b))
	}
	if b[0] != minFeerateBucketKoinuPerKB {
		t.Fatalf("first %d", b[0])
	}
	if feerateBucketIndex(b, 50_000) != 0 {
		t.Fatalf("low bucket index")
	}
}

func TestTxConfirmStatsEstimateConservativeVsEconomical(t *testing.T) {
	s := NewTxConfirmStats()
	for i := 0; i < 40; i++ {
		s.RecordConfirm(1, 500_000)
		s.FlushBlock()
	}
	for i := 0; i < 40; i++ {
		s.RecordConfirm(6, 80_000)
		s.FlushBlock()
	}
	con := s.Estimate(6, true)
	eco := s.Estimate(6, false)
	if con == 0 {
		t.Fatal("conservative estimate")
	}
	if eco == 0 {
		t.Fatal("economical estimate")
	}
	if con < eco {
		t.Fatalf("conservative %d < economical %d", con, eco)
	}
}

func TestFeeHistoryConfirmStatsIntegration(t *testing.T) {
	h := NewFeeHistory(10)
	for i := 0; i < 12; i++ {
		h.RecordMempoolConfirmedSamples([]MempoolConfirmSample{
			{FeeratePerKB: 400_000, BlocksWaited: 1},
			{FeeratePerKB: 420_000, BlocksWaited: 1},
		})
	}
	for i := 0; i < 12; i++ {
		h.RecordMempoolConfirmedSamples([]MempoolConfirmSample{
			{FeeratePerKB: 90_000, BlocksWaited: 6},
		})
	}
	if got := h.EstimatePerKBFromConfirmStats(6, true); got == 0 {
		t.Fatal("confirm stats estimate")
	}
	stats := h.ConfirmStatsBucketMarket()
	if len(stats) == 0 {
		t.Fatal("market stats")
	}
}
