// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestMainnetFieldDiskConnectCases(t *testing.T) {
	cases := MainnetFieldDiskConnectCases(2)
	if len(cases) != 0 {
		t.Fatalf("tip=2: got %d cases", len(cases))
	}
	cases = MainnetFieldDiskConnectCases(3)
	if len(cases) != 1 || cases[0].End != 3 {
		t.Fatalf("tip=3: %#v", cases)
	}
	cases = MainnetFieldDiskConnectCases(272)
	if len(cases) != 3 {
		t.Fatalf("tip=272: got %d want 3", len(cases))
	}
	if cases[len(cases)-1].End != 272 {
		t.Fatalf("last tier %#v", cases[len(cases)-1])
	}
	cases = MainnetFieldDiskConnectCases(3368)
	if len(cases) != 6 {
		t.Fatalf("tip=3368: got %d want 6", len(cases))
	}
	if cases[len(cases)-1].End != 3368 {
		t.Fatalf("last tier %#v", cases[len(cases)-1])
	}
}
