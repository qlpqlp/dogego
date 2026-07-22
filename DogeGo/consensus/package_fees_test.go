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

func TestPackageFeeReportForTxAncestorPackage(t *testing.T) {
	pool := mempool.New(50)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000, PkScript: p2pkhScript()}},
	}
	if err := pool.Add(parent.SerializeForHash()); err != nil {
		t.Fatal(err)
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 9_000_000, PkScript: p2pkhScript()}},
	}
	view := stubPrevOutView{outpointKey(parent.TxHash(), 0): {Value: parent.Vout[0].Value, PkScript: parent.Vout[0].PkScript}}
	report, err := PackageFeeReportForTx(child, pool, view)
	if err != nil {
		t.Fatal(err)
	}
	if report.BaseFeeKoinu != 1_000_000 {
		t.Fatalf("base %d", report.BaseFeeKoinu)
	}
	if report.AncestorFeeKoinu != 1_000_000 {
		t.Fatalf("ancestor %d", report.AncestorFeeKoinu)
	}
}
