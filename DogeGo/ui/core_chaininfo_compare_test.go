// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

func TestCompareChainInfoConsensusFieldsMatch(t *testing.T) {
	dg := map[string]any{
		"bestblockhash": "abc",
		"chainwork":     "000abc",
		"mediantime":    float64(1_700_000_000),
	}
	core := map[string]any{
		"bestblockhash": "abc",
		"chainwork":     "000abc",
		"mediantime":    float64(1_700_000_000),
	}
	out := &CoreCompareResult{}
	if !compareChainInfoConsensusFields(out, dg, core) {
		t.Fatal("expected consensus fields match")
	}
	if len(out.Fields) != 2 {
		t.Fatalf("fields=%+v", out.Fields)
	}
}

func TestCompareChainInfoConsensusFieldsChainworkMismatchSharedTip(t *testing.T) {
	dg := map[string]any{"bestblockhash": "abc", "chainwork": "001", "mediantime": float64(100)}
	core := map[string]any{"bestblockhash": "abc", "chainwork": "002", "mediantime": float64(100)}
	out := &CoreCompareResult{}
	if compareChainInfoConsensusFields(out, dg, core) {
		t.Fatal("expected chainwork mismatch to fail")
	}
	for _, f := range out.Fields {
		if f.Name == "chainwork" && f.Match {
			t.Fatalf("expected chainwork mismatch, got %+v", f)
		}
	}
}

func TestCompareChainInfoConsensusFieldsTipDiffLenient(t *testing.T) {
	dg := map[string]any{"bestblockhash": "abc", "chainwork": "001", "mediantime": float64(100)}
	core := map[string]any{"bestblockhash": "def", "chainwork": "002", "mediantime": float64(200)}
	out := &CoreCompareResult{}
	if !compareChainInfoConsensusFields(out, dg, core) {
		t.Fatal("expected lenient match when tips differ")
	}
}

func TestSharedChainTip(t *testing.T) {
	if !sharedChainTip(map[string]any{"bestblockhash": "x"}, map[string]any{"bestblockhash": "x"}) {
		t.Fatal("expected shared tip")
	}
	if sharedChainTip(map[string]any{"bestblockhash": "x"}, map[string]any{"bestblockhash": "y"}) {
		t.Fatal("expected different tips")
	}
}

func TestAppendVerificationProgressCompareCaughtUp(t *testing.T) {
	dg := map[string]any{
		"initialblockdownload": false,
		"verificationprogress": 0.9999,
	}
	core := map[string]any{
		"initialblockdownload": false,
		"verificationprogress": 1.0,
	}
	out := &CoreCompareResult{}
	appendVerificationProgressCompare(out, dg, core)
	if len(out.Fields) != 1 || !out.Fields[0].Match {
		t.Fatalf("expected match, got %+v", out.Fields)
	}
}

func TestAppendVerificationProgressCompareSkippedDuringIBD(t *testing.T) {
	dg := map[string]any{
		"initialblockdownload": true,
		"verificationprogress": 0.5,
	}
	core := map[string]any{
		"initialblockdownload": false,
		"verificationprogress": 1.0,
	}
	out := &CoreCompareResult{}
	appendVerificationProgressCompare(out, dg, core)
	if len(out.Fields) != 0 {
		t.Fatalf("expected skip during IBD, got %+v", out.Fields)
	}
}
