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

type memRawPool struct {
	p *mempool.Pool
}

func (m memRawPool) MempoolRawByDisplayTxid(rpcTxid string) ([]byte, bool) {
	return m.p.MempoolRawByDisplayTxid(rpcTxid)
}

func (m memRawPool) BlocksWaitedAtConfirm(displayTxid string, confirmHeight int64) int {
	return m.p.BlocksWaitedAtConfirm(displayTxid, confirmHeight)
}

func TestCollectMempoolConfirmedSamples(t *testing.T) {
	p := mempool.New(0)
	p.SetTipHeightFn(func() int64 { return 10 })
	prev := [32]byte{9}
	view := stubPrevOutView{outpointKey(prev, 0): {Value: 1e8, PkScript: []byte{0x51}}}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 99e6, PkScript: []byte{0x51}}},
	}
	raw, err := spend.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	pb := &wire.ParsedBlock{
		Txs: []*wire.Tx{
			{Version: 1, Vin: []wire.TxIn{{PrevIdx: 0xffffffff}}, Vout: []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}}},
			spend,
		},
	}
	samples := CollectMempoolConfirmedSamples(pb, memRawPool{p}, view, 12)
	if len(samples) != 1 || samples[0].FeeratePerKB == 0 {
		t.Fatalf("samples=%v", samples)
	}
	if samples[0].BlocksWaited != 3 { // admitted at 10, confirmed at 12 -> 3 blocks
		t.Fatalf("blocks waited=%d want 3", samples[0].BlocksWaited)
	}
}

func TestFeeHistoryConfirmBuckets(t *testing.T) {
	h := NewFeeHistory(8)
	h.RecordMempoolConfirmedSamples([]MempoolConfirmSample{
		{FeeratePerKB: 100_000, BlocksWaited: 2},
		{FeeratePerKB: 200_000, BlocksWaited: 6},
	})
	if got := h.EstimateMempoolConfirmedPerKB(2); got != 100_000 {
		t.Fatalf("target 2: got %d", got)
	}
	stats := h.MempoolConfirmBucketStats()
	if stats["2"]["samples"].(int) != 1 {
		t.Fatalf("stats=%v", stats)
	}
}
