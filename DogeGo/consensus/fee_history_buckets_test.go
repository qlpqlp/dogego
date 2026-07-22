// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEstimatePerKBFromBucketMedians(t *testing.T) {
	h := NewFeeHistory(32)
	h.Record([]uint64{200_000, 220_000})
	h.Record([]uint64{300_000})
	got := h.EstimatePerKBFromBucketMedians(6)
	if got < 200_000 {
		t.Fatalf("got %d", got)
	}
}
