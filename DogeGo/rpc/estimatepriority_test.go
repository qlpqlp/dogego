// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"math"
	"testing"

	"dogego/consensus"
)

func TestEstimatePriorityAlwaysNegativeOne(t *testing.T) {
	raw, _ := json.Marshal(6)
	res, code, msg := execEstimatePriority(nil, []json.RawMessage{raw})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	f, ok := res.(float64)
	if !ok || f != -1 {
		t.Fatalf("got %v", res)
	}
}

func TestEstimateSmartPriorityReturnsInfWhenMinRelayEnforced(t *testing.T) {
	raw, _ := json.Marshal(6)
	res, code, msg := execEstimateSmartPriority(nil, []json.RawMessage{raw})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("got %T", res)
	}
	p, ok := m["priority"].(float64)
	if !ok || p != consensus.InfPriority {
		t.Fatalf("priority %v want %v", m["priority"], consensus.InfPriority)
	}
	if m["blocks"].(int) != 6 {
		t.Fatalf("blocks %v", m["blocks"])
	}
}

func TestEstimateSmartPriorityInfPriorityMatchesCoreFormula(t *testing.T) {
	want := 1e9 * float64(consensus.MaxMoney)
	if math.Abs(consensus.InfPriority-want) > 0 {
		t.Fatalf("InfPriority %v want %v", consensus.InfPriority, want)
	}
}
