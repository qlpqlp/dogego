// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"
)

func TestCompareBlockFilterWithCoreMatch(t *testing.T) {
	ep := CoreParityEndpoints{Addr: "http://127.0.0.1:1"}
	out := &CoreMaintenanceResult{}
	compareBlockFilterWithCore(out, ep, "abc", map[string]any{"filter": "00"})
	if len(out.Notes) == 0 || out.Notes[0] != "core_getblockfilter_compare_skipped" {
		t.Fatalf("expected skip without core, notes=%v checks=%+v", out.Notes, out.Checks)
	}
}

func TestCompareBlockFilterWithCoreMismatchWarning(t *testing.T) {
	out := &CoreMaintenanceResult{}
	compareBlockFilterWithCore(out, CoreParityEndpoints{}, "", nil)
	if len(out.Checks) != 0 {
		t.Fatal("expected no checks for empty input")
	}
}
