// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"testing"

	"dogego/mempool"
	"dogego/primitives"
	"dogego/wire"
)

func buildBlockRawForMempoolConfirm(t *testing.T, txs ...*wire.Tx) []byte {
	t.Helper()
	if len(txs) == 0 {
		t.Fatal("need txs")
	}
	merkle := txs[0].TxHash()
	for i := 1; i < len(txs); i++ {
		merkle = wire.HashPair(merkle, txs[i].TxHash())
	}
	hdr := primitives.BlockHeader{
		Version: 1, MerkleRoot: merkle, Timestamp: 1747000000,
		Bits: 0x1e0ffff0, Nonce: 1,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, uint64(len(txs)))
	for _, tx := range txs {
		ser, err := tx.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = block.Write(ser)
	}
	return block.Bytes()
}

func TestCollectMempoolConfirmedSamplesRaw(t *testing.T) {
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
	cb := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
	}
	blockRaw := buildBlockRawForMempoolConfirm(t, cb, spend)
	samples := CollectMempoolConfirmedSamplesRaw(blockRaw, memRawPool{p}, view, 12)
	if len(samples) != 1 || samples[0].FeeratePerKB == 0 {
		t.Fatalf("samples=%v", samples)
	}
	if samples[0].BlocksWaited != 3 {
		t.Fatalf("blocks waited=%d want 3", samples[0].BlocksWaited)
	}
}
