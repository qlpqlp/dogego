// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestTipBackfillRange(t *testing.T) {
	tests := []struct {
		tip    int64
		max    int
		wantLo int64
		wantOk bool
	}{
		{tip: 0, max: 5, wantOk: false},
		{tip: 1, max: 5, wantLo: 1, wantOk: true},
		{tip: 3, max: 5, wantLo: 1, wantOk: true},
		{tip: 5, max: 5, wantLo: 1, wantOk: true},
		{tip: 10, max: 5, wantLo: 6, wantOk: true},
		{tip: 100, max: 1, wantLo: 100, wantOk: true},
		{tip: 100, max: 0, wantOk: false},
	}
	for _, tc := range tests {
		lo, ok := tipBackfillRange(tc.tip, tc.max)
		if ok != tc.wantOk || lo != tc.wantLo {
			t.Fatalf("tip=%d max=%d: got (%d,%v) want (%d,%v)", tc.tip, tc.max, lo, ok, tc.wantLo, tc.wantOk)
		}
		if ok && lo > tc.tip {
			t.Fatalf("tip=%d max=%d: start %d > tip", tc.tip, tc.max, lo)
		}
	}
}
