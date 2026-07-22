// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEstimateFeeFromRates(t *testing.T) {
	rates := []uint64{100_000, 200_000, 400_000, 800_000}
	if got := EstimateFeeFromRates(rates, 1); got != 800_000 {
		t.Fatalf("nblocks=1: got %d want max", got)
	}
	if got := EstimateFeeFromRates(rates, 6); got != 400_000 {
		t.Fatalf("nblocks=6: got %d want 75th pct", got)
	}
}
