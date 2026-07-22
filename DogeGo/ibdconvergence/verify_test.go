// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ibdconvergence

import "testing"

func TestCompareSnapshotsForwardProgress(t *testing.T) {
	h0, h1 := int64(1000), int64(1000)
	c0, c1 := int64(10), int64(12)
	b0, b1 := int64(8), int64(10)
	out := CompareSnapshots(VerifyResult{
		T0: ProgressSnapshot{Source: "rpc", Headers: &h0, Contiguous: &c0, Blocks: &b0},
		T1: ProgressSnapshot{Source: "rpc", Headers: &h1, Contiguous: &c1, Blocks: &b1},
	}, Options{
		IntervalSec:            60,
		MinContiguousAdvance:   1,
		MinBlocksAdvance:       1,
		MinRawProbeAdvance:     1,
		MaxContiguousRegression: 64,
	})
	if !out.OK {
		t.Fatalf("expected ok: %+v", out)
	}
	if out.ContiguousAdvance != 2 || out.BlockAdvance != 2 {
		t.Fatalf("advances: %+v", out)
	}
}

func TestCompareSnapshotsRegression(t *testing.T) {
	c0, c1 := int64(100), int64(20)
	out := CompareSnapshots(VerifyResult{
		T0: ProgressSnapshot{Contiguous: &c0},
		T1: ProgressSnapshot{Contiguous: &c1},
	}, Options{MaxContiguousRegression: 64})
	if out.OK || len(out.Issues) == 0 {
		t.Fatalf("expected regression fail: %+v", out)
	}
}

func TestCompareSnapshotsBodyOnlyInFlight(t *testing.T) {
	cont := int64(500)
	blocks := int64(500)
	inFlight := int64(3)
	out := CompareSnapshots(VerifyResult{
		T0: ProgressSnapshot{Contiguous: &cont, Blocks: &blocks},
		T1: ProgressSnapshot{Contiguous: &cont, Blocks: &blocks, RawInFlight: &inFlight},
	}, Options{
		MinContiguousAdvance: 1,
		MinBlocksAdvance:     1,
		MinRawProbeAdvance:   1,
	})
	if !out.OK {
		t.Fatalf("expected body-only ok: %+v", out)
	}
}
