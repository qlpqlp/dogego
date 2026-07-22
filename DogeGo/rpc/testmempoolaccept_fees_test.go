// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/consensus"
	"dogego/mempool"
	"dogego/wire"
)

func TestApplyMempoolAcceptFeesModifiedDelta(t *testing.T) {
	res := map[string]interface{}{
		"fees": map[string]interface{}{
			"base": 0.0, "modified": 0.0, "ancestor": 0.0, "descendant": 0.0, "effective-feerate": 0.0,
		},
	}
	e := mempool.MemPoolVerboseEntry{TxID: "aa"}
	_ = e
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: []byte{0x51}}},
	}
	// No view → fees stay zero.
	if eff := applyMempoolAcceptFees(res, tx, nil, nil, nil, "aa"); eff != 0 {
		t.Fatalf("eff=%v", eff)
	}
	fees := res["fees"].(map[string]interface{})
	if fees["base"].(float64) != 0 {
		t.Fatalf("base=%v", fees["base"])
	}

	// Direct fee object shape when report is injected via stub is not available without view;
	// verify helper math via mempoolEntryJSON parity fields.
	report := consensus.PackageFeeReport{
		BaseFeeKoinu:       1_000_000,
		AncestorFeeKoinu:   2_000_000,
		DescendantFeeKoinu: 3_000_000,
		EffectiveRatePerKB: 400_000,
	}
	m := mempoolEntryJSON(mempool.MemPoolVerboseEntry{}, report, mempool.PackageStats{}, false, 50_000)
	f := m["fees"].(map[string]interface{})
	if f["modified"].(float64) != 0.0105 {
		t.Fatalf("modified=%v", f["modified"])
	}
	if f["effective-feerate"].(float64) != 0.004 {
		t.Fatalf("eff=%v", f["effective-feerate"])
	}
}
