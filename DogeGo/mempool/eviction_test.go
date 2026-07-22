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

func TestAddWithEviction(t *testing.T) {
	pool := New(2)
	a := &wire.Tx{Version: 1, Vin: []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}}, Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}}}
	b := &wire.Tx{Version: 1, Vin: []wire.TxIn{{PrevHash: [32]byte{2}, PrevIdx: 0, Sequence: 0xffffffff}}, Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}}}
	c := &wire.Tx{Version: 1, Vin: []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}}, Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}}}
	ar, _ := a.Serialize()
	br, _ := b.Serialize()
	cr, _ := c.Serialize()
	aid := TxIDDisplayHex(a.TxHash())
	bid := TxIDDisplayHex(b.TxHash())
	cid := TxIDDisplayHex(c.TxHash())
	if err := pool.Add(ar); err != nil {
		t.Fatal(err)
	}
	if err := pool.Add(br); err != nil {
		t.Fatal(err)
	}
	fees := map[string]int64{aid: 100, bid: 500}
	sizes := map[string]int{aid: 100, bid: 100, cid: 100}
	if err := pool.AddWithEviction(cr, fees, sizes); err != nil {
		t.Fatal(err)
	}
	if pool.Count() != 2 {
		t.Fatalf("count %d", pool.Count())
	}
	if pool.ContainsTxID(aid) {
		t.Fatal("expected low-fee tx evicted")
	}
	if !pool.ContainsTxID(bid) {
		t.Fatal("expected high-fee tx kept")
	}
}
