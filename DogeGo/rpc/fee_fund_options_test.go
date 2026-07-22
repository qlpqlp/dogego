// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"
)

func TestFundFeePerKBFromOptionsConfTarget(t *testing.T) {
	paths := &DataPaths{
		MempoolMinRelayFee: func() uint64 { return 100_000 },
	}
	opts := map[string]json.RawMessage{
		"conf_target": json.RawMessage(`6`),
	}
	rate, code, msg := fundFeePerKBFromOptions(paths, opts)
	if code != 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	if rate != 100_000 {
		t.Fatalf("rate=%d want min relay 100000", rate)
	}
}

func TestFundFeePerKBFromOptionsFeeRateWins(t *testing.T) {
	paths := &DataPaths{
		MempoolMinRelayFee: func() uint64 { return 100_000 },
	}
	opts := map[string]json.RawMessage{
		"conf_target": json.RawMessage(`6`),
		"fee_rate":    json.RawMessage(`2.5`),
	}
	rate, code, msg := fundFeePerKBFromOptions(paths, opts)
	if code != 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	want := uint64(250_000_000)
	if rate != want {
		t.Fatalf("rate=%d want %d", rate, want)
	}
}

func TestFundFeePerKBFromOptionsBadEstimateMode(t *testing.T) {
	opts := map[string]json.RawMessage{
		"conf_target":   json.RawMessage(`6`),
		"estimate_mode": json.RawMessage(`"fast"`),
	}
	_, code, msg := fundFeePerKBFromOptions(nil, opts)
	if code != -8 || msg == "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}
