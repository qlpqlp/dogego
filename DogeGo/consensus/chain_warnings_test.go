// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/chain"
)

func TestUnexpectedBlockVersionWarning(t *testing.T) {
	j := &vbJournal{headers: make([]header80, 3)}
	for i := range j.headers {
		prev := int64(i) - 1
		j.headers[i] = header80{version: ComputeBlockVersion(j, chain.MainnetDogecoin, prev), time: 1_500_000_000}
	}
	j.headers[2].version = 0xffffffff
	w := unexpectedBlockVersionWarning(j, chain.MainnetDogecoin, 2)
	if len(w) == 0 {
		t.Fatal("expected warning for unexpected version")
	}
}

func TestFeeHistoryRecord(t *testing.T) {
	h := NewFeeHistory(2)
	h.Record([]uint64{100_000, 200_000})
	h.Record([]uint64{300_000})
	if got := h.EstimatePerKB(1); got != 300_000 {
		t.Fatalf("got %d", got)
	}
}
