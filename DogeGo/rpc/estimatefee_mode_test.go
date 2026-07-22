// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/consensus"
)

func TestSmartFeeConservativeVsEconomical(t *testing.T) {
	h := consensus.NewFeeHistory(10)
	h.Record([]uint64{50_000})
	h.Record([]uint64{500_000})
	paths := &DataPaths{
		ConfirmedFeeEstimate: func(nblocks int) uint64 {
			return h.EstimatePerKBEconomical(nblocks)
		},
		ConfirmedFeeEstimateConservative: func(nblocks int) uint64 {
			return h.EstimatePerKBConservative(nblocks)
		},
	}
	eco, _, _ := smartFeeKoinuPerKB(paths, 6, false)
	con, _, _ := smartFeeKoinuPerKB(paths, 6, true)
	if con < eco {
		t.Fatalf("conservative %d < economical %d", con, eco)
	}
}

func TestExecEstimateSmartFeeModeParam(t *testing.T) {
	paths := &DataPaths{}
	raw, _ := json.Marshal("economical")
	res, code, msg := execEstimateSmartFee(paths, []json.RawMessage{json.RawMessage(`6`), raw})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	_, code, msg = execEstimateSmartFee(paths, []json.RawMessage{json.RawMessage(`6`), json.RawMessage(`"invalid"`)})
	if code != -8 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	_ = res
}
