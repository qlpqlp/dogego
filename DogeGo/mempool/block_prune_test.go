// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"testing"

	"dogego/wire"
)

func TestRemoveForBlock(t *testing.T) {
	pool := New(10)
	a := &wire.Tx{Version: 1, Vin: []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}}, Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}}}
	b := &wire.Tx{Version: 1, Vin: []wire.TxIn{{PrevHash: [32]byte{2}, PrevIdx: 0, Sequence: 0xffffffff}}, Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}}}
	conflict := &wire.Tx{Version: 1, Vin: []wire.TxIn{{PrevHash: a.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}}, Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x52}}}}
	ar, _ := a.Serialize()
	br, _ := b.Serialize()
	cr, _ := conflict.Serialize()
	_ = pool.Add(ar)
	_ = pool.Add(br)
	_ = pool.Add(cr)

	pb := &wire.ParsedBlock{
		Txs: []*wire.Tx{
			{Version: 1, Vin: []wire.TxIn{{Sequence: 0xffffffff}}, Vout: []wire.TxOut{{Value: 50, PkScript: []byte{0x51}}}},
			a,
		},
	}
	removed := pool.RemoveForBlock(pb)
	if pool.ContainsTxID(txidDisplayHex(a.TxHash())) {
		t.Fatal("block tx should be removed from mempool")
	}
	if pool.ContainsTxID(txidDisplayHex(conflict.TxHash())) {
		t.Fatal("conflict should be removed")
	}
	if !pool.ContainsTxID(txidDisplayHex(b.TxHash())) {
		t.Fatalf("unrelated tx should remain; removed %v", removed)
	}
}
