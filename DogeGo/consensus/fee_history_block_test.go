// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/wire"
)

func TestRecordBlockConnectedSkipsMempoolConfirmedTxIDs(t *testing.T) {
	h := NewFeeHistory(8)
	h.confirmStats.SetBestSeenHeight(10)
	h.lastMempoolConfirmedTxIDs = map[string]struct{}{"already": {}}

	h.mu.Lock()
	samples := []BlockFeeSample{
		{TxID: "already", FeeratePerKB: 500_000},
		{TxID: "newtx", FeeratePerKB: 100_000},
	}
	for _, s := range samples {
		if _, ok := h.lastMempoolConfirmedTxIDs[s.TxID]; ok {
			continue
		}
		h.confirmStats.RecordConfirm(1, s.FeeratePerKB)
	}
	h.confirmStats.FlushBlock()
	h.lastMempoolConfirmedTxIDs = nil
	h.mu.Unlock()

	bi := feerateBucketIndex(h.confirmStats.buckets, 500_000)
	if h.confirmStats.curConf[0][bi] > 0 {
		t.Fatal("mempool-confirmed tx should not get duplicate 1-block cur confirm")
	}
	biNew := feerateBucketIndex(h.confirmStats.buckets, 100_000)
	if h.confirmStats.txCtAvg[biNew] == 0 && h.confirmStats.confAvg[0][biNew] == 0 {
		t.Fatal("untracked block tx should update confirm stats after flush")
	}
}

func TestRecordBlockConnectedFlushOnEmptySamples(t *testing.T) {
	h := NewFeeHistory(8)
	h.confirmStats.SetBestSeenHeight(5)
	h.confirmStats.RecordConfirm(1, 50_000)
	h.RecordBlockConnected(&wire.ParsedBlock{}, MultiPrevOutView{})
	if h.confirmStats.curTxCt[0] != 0 {
		t.Fatal("cur batch should be cleared after flush")
	}
}

func TestRecordMempoolConfirmedSetsDedupeSet(t *testing.T) {
	h := NewFeeHistory(8)
	h.confirmStats.SetBestSeenHeight(3)
	h.RecordMempoolConfirmedSamples([]MempoolConfirmSample{
		{TxID: "x", FeeratePerKB: 10_000, BlocksWaited: 1},
	})
	if _, ok := h.lastMempoolConfirmedTxIDs["x"]; !ok {
		t.Fatal("expected dedupe set")
	}
}
