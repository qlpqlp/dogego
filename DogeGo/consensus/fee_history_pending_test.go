// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/mempool"
	"dogego/wire"
)

func TestFeeHistoryPendingTrackConfirm(t *testing.T) {
	h := NewFeeHistory(8)
	h.TrackMempoolAdmission("aa", 50_000, 10)
	if h.PendingMempoolFeeTracks() != 1 {
		t.Fatalf("pending=%d", h.PendingMempoolFeeTracks())
	}
	h.RecordMempoolConfirmedSamples([]MempoolConfirmSample{
		{TxID: "aa", FeeratePerKB: 99_000, BlocksWaited: 2},
	})
	if h.PendingMempoolFeeTracks() != 0 {
		t.Fatalf("still pending")
	}
	if got := h.EstimateMempoolConfirmedPerKB(2); got != 50_000 {
		t.Fatalf("used admission feerate: got %d", got)
	}
}

func TestFeeHistoryLeftWithoutConfirm(t *testing.T) {
	h := NewFeeHistory(8)
	h.TrackMempoolAdmission("bb", 80_000, 5)
	h.RecordMempoolLeftWithoutConfirm("bb", 20)
	if h.PendingMempoolFeeTracks() != 0 {
		t.Fatal("pending should clear")
	}
	if h.LeftWithoutConfirmCount() != 1 {
		t.Fatalf("left=%d", h.LeftWithoutConfirmCount())
	}
	stats := h.MempoolLeftBucketStats()
	if stats["12"]["samples"].(int) != 1 {
		t.Fatalf("left buckets=%v", stats)
	}
	if got := h.EstimateLeftWithoutConfirmPerKB(12); got != 80_000 {
		t.Fatalf("got %d", got)
	}
}

func TestPoolOnRemoveRecordsLeftWithoutConfirm(t *testing.T) {
	h := NewFeeHistory(8)
	p := mempool.New(0)
	p.SetOnRemove(func(id string) { h.RecordMempoolLeftWithoutConfirm(id, 10) })
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Script: []byte{0x01}}},
		Vout:    []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	id := displayTxHash(tx.TxHash())
	h.TrackMempoolAdmission(id, 10_000, 1)
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	if !p.RemoveByTxID(id) {
		t.Fatal("remove failed")
	}
	if h.PendingMempoolFeeTracks() != 0 {
		t.Fatal("expected pending cleared on remove")
	}
	if h.LeftWithoutConfirmCount() != 1 {
		t.Fatal("expected left-without-confirm sample")
	}
}
