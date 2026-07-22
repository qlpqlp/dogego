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
)

func TestMempoolEntryModifiedFeeWithDelta(t *testing.T) {
	e := mempool.MemPoolVerboseEntry{TxID: "aa", Size: 250, VSize: 250}
	report := consensus.PackageFeeReport{BaseFeeKoinu: 1_000_000}
	m := mempoolEntryJSON(e, report, mempool.PackageStats{}, false, 50_000)
	if m["fee"].(float64) != 0.01 {
		t.Fatalf("fee=%v", m["fee"])
	}
	if m["modifiedfee"].(float64) != 0.0105 {
		t.Fatalf("modifiedfee=%v", m["modifiedfee"])
	}
	fees := m["fees"].(map[string]interface{})
	if fees["modified"].(float64) != 0.0105 {
		t.Fatalf("fees.modified=%v", fees["modified"])
	}
}
