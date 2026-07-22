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

func TestConflictPackageFeeSizeUsesDescendants(t *testing.T) {
	pool := New(10)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000, PkScript: []byte{0x51}}},
	}
	conflict := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parent.TxHash(),
			PrevIdx:  0,
			Sequence: 0xfffffffd,
		}},
		Vout: []wire.TxOut{{Value: 800_000, PkScript: []byte{0x51}}},
	}
	child := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: conflict.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 500_000, PkScript: []byte{0x52}}},
	}
	for _, tx := range []*wire.Tx{parent, conflict, child} {
		raw, err := tx.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if err := pool.Add(raw); err != nil {
			t.Fatal(err)
		}
	}
	pid := TxIDDisplayHex(parent.TxHash())
	cid := TxIDDisplayHex(conflict.TxHash())
	kid := TxIDDisplayHex(child.TxHash())
	fees := map[string]int64{pid: 10, cid: 100, kid: 1000}
	sizes, err := pool.BuildMempoolSizes()
	if err != nil {
		t.Fatal(err)
	}
	fee, size, ok := pool.ConflictPackageFeeSize(cid, fees, sizes)
	if !ok {
		t.Fatal("ConflictPackageFeeSize failed")
	}
	if fee != 1100 {
		t.Fatalf("conflict descendant fees=%d want 1100 (conflict+child; parent excluded)", fee)
	}
	wantSize := sizes[cid] + sizes[kid]
	if size != wantSize {
		t.Fatalf("conflict descendant size=%d want %d", size, wantSize)
	}
	st, err := pool.PackageStatsForTxID(cid, fees, sizes)
	if err != nil {
		t.Fatal(err)
	}
	if st.AncestorFeesKoinu != 110 {
		t.Fatalf("sanity ancestor fees=%d", st.AncestorFeesKoinu)
	}
	if fee == st.AncestorFeesKoinu {
		t.Fatal("ConflictPackageFeeSize must not use ancestor package")
	}
}
