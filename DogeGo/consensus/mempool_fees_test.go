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

func TestBuildMempoolFeeRatesAndTotal(t *testing.T) {
	var prev [32]byte
	prev[0] = 5
	view := stubFeeView{outpointKey(prev, 0): {Value: 1000, PkScript: []byte{0x51}}}
	pool := mempool.New(10)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 400, PkScript: []byte{0x51}}},
	}
	raw, _ := tx.Serialize()
	_ = pool.Add(raw)
	rates := BuildMempoolFeeRates(pool, view)
	if len(rates) != 1 {
		t.Fatalf("rates %d", len(rates))
	}
	for _, r := range rates {
		if r <= 0 {
			t.Fatalf("rate %d", r)
		}
	}
	if TotalMempoolFeesKoinu(pool, view) != 600 {
		t.Fatalf("total %d", TotalMempoolFeesKoinu(pool, view))
	}
}

type stubFeeView map[[36]byte]PrevOut

func (s stubFeeView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	o, ok := s[outpointKey(prevHash, idx)]
	return o, ok
}
