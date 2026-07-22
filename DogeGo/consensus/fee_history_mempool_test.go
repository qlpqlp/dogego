// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"
)

func TestFeeHistoryMempoolConfirmed(t *testing.T) {
	h := NewFeeHistory(4)
	h.RecordMempoolConfirmed([]uint64{50_000})
	h.RecordMempoolConfirmed([]uint64{100_000, 200_000})
	if got := h.EstimateMempoolConfirmedPerKB(1); got != 200_000 {
		t.Fatalf("depth 1: got %d", got)
	}
	if got := h.EstimatePerKBConservative(1); got < 200_000 {
		t.Fatalf("conservative: got %d", got)
	}
}

func TestFeeHistoryMempoolConfirmedEconomical(t *testing.T) {
	h := NewFeeHistory(8)
	h.RecordMempoolConfirmedSamples([]MempoolConfirmSample{
		{FeeratePerKB: 100_000, BlocksWaited: 1},
		{FeeratePerKB: 400_000, BlocksWaited: 1},
	})
	if got := h.EstimateMempoolConfirmedPerKBEconomical(1); got != 100_000 {
		t.Fatalf("economical: got %d", got)
	}
	if got := h.EstimateMempoolConfirmedPerKB(1); got != 400_000 {
		t.Fatalf("conservative-ish: got %d", got)
	}
}

func TestFeeHistoryMempoolConfirmedPersist(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/fee_history.json"
	h := NewFeeHistory(8)
	h.RecordMempoolConfirmed([]uint64{42_000})
	if err := h.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFeeHistoryFile(path, 8)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.EstimateMempoolConfirmedPerKB(1); got != 42_000 {
		t.Fatalf("loaded: got %d", got)
	}
}
