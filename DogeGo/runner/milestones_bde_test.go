// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"strings"
	"testing"
)

func TestMilestonesBDESuitesNonEmpty(t *testing.T) {
	if len(milestoneBSuites()) == 0 || len(milestoneDSuites()) == 0 || len(milestoneESuites()) == 0 {
		t.Fatal("milestone suite lists must be non-empty")
	}
}

func TestMilestonesBDESummaryLine(t *testing.T) {
	ok := MilestonesBDESummaryLine(MilestonesBDEResult{OK: true})
	if !strings.Contains(ok, "PASS") {
		t.Fatalf("summary=%q", ok)
	}
	fail := MilestonesBDESummaryLine(MilestonesBDEResult{OK: false})
	if !strings.Contains(fail, "FAIL") {
		t.Fatalf("summary=%q", fail)
	}
}
