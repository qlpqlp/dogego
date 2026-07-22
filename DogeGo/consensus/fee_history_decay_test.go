// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestFeeHistoryDecayFavorsRecent(t *testing.T) {
	h := NewFeeHistory(10)
	h.Record([]uint64{10_000})
	h.Record([]uint64{800_000})
	if got := h.EstimatePerKBDecay(6); got < 400_000 {
		t.Fatalf("decay estimate %d expected closer to recent 800k", got)
	}
}
