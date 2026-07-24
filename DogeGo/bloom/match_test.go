// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package bloom

import (
	"testing"

	"dogego/wire"
)

func TestMatchRelevantTx(t *testing.T) {
	f, err := NewEmpty(20, 0.00001, 0, UpdateAll)
	if err != nil {
		t.Fatal(err)
	}
	script := []byte{0x76, 0xa9, 0x14, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 0x88, 0xac}
	f.Insert(script)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1000, PkScript: script}},
	}
	if !MatchRelevantTx(f, tx) {
		t.Fatal("want match on pkScript")
	}
	txid := tx.TxHash()
	if !f.ContainsOutpoint(txid, 0) {
		t.Fatal("UpdateAll should insert matched outpoint")
	}
}
