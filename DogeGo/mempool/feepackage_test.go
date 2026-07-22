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

func TestPackageStatsForTxID(t *testing.T) {
	pool := New(10)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	child := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parent.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x52}}},
	}
	pr, _ := parent.Serialize()
	cr, _ := child.Serialize()
	_ = pool.Add(pr)
	_ = pool.Add(cr)
	pid := TxIDDisplayHex(parent.TxHash())
	cid := TxIDDisplayHex(child.TxHash())
	fees := map[string]int64{pid: 100, cid: 200}
	sizes, _ := pool.BuildMempoolSizes()
	st, err := pool.PackageStatsForTxID(cid, fees, sizes)
	if err != nil {
		t.Fatal(err)
	}
	if st.AncestorCount != 2 || st.DescendantCount != 1 {
		t.Fatalf("counts anc=%d desc=%d", st.AncestorCount, st.DescendantCount)
	}
	if st.AncestorFeesKoinu != 300 || st.DescendantFeesKoinu != 200 {
		t.Fatalf("fees anc=%d desc=%d", st.AncestorFeesKoinu, st.DescendantFeesKoinu)
	}
}
