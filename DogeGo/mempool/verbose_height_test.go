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

func TestMemPoolVerboseEntryHeight(t *testing.T) {
	p := New(0)
	p.SetTipHeightFn(func() int64 { return 42 })
	raw := testMinimalCoinbaseBytes(t)
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	id := TxIDDisplayHex(tx.TxHash())
	entries, err := p.SortedMemPoolVerbose()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Height != 42 {
		t.Fatalf("entries=%v", entries)
	}
	e, err := p.MemPoolVerboseEntryForTxID(id)
	if err != nil || e.Height != 42 {
		t.Fatalf("entry height=%d err=%v", e.Height, err)
	}
}

func testMinimalCoinbaseBytes(t *testing.T) []byte {
	t.Helper()
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Script: []byte{0x01}}},
		Vout:    []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
