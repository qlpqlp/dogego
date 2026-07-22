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

func TestEstimateFeeOneBlockReturnsNegativeOne(t *testing.T) {
	raw, _ := json.Marshal(1)
	res, code, msg := execEstimateFee(nil, []json.RawMessage{raw})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	f, ok := res.(float64)
	if !ok || f != -1 {
		t.Fatalf("got %v", res)
	}
}

func TestEstimateSmartFeeOneBlockTargetsTwo(t *testing.T) {
	raw, _ := json.Marshal(1)
	res, code, msg := execEstimateSmartFee(nil, []json.RawMessage{raw})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("got %T", res)
	}
	blocks, _ := m["blocks"].(int)
	if blocks != 2 {
		t.Fatalf("blocks %v want 2", m["blocks"])
	}
	want := float64(consensus.MinRelayTxFeePerKB()) / 1e8
	if m["feerate"].(float64) != want {
		t.Fatalf("feerate %v want %v", m["feerate"], want)
	}
}
