// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "testing"

func TestContiguousReconcileProgressLifecycle(t *testing.T) {
	EndContiguousReconcile()
	if _, ok := ContiguousReconcileProgress(); ok {
		t.Fatal("expected inactive")
	}
	BeginContiguousReconcile()
	defer EndContiguousReconcile()
	st, ok := ContiguousReconcileProgress()
	if !ok || !st.Active || st.Percent != 0 {
		t.Fatalf("begin: %#v ok=%v", st, ok)
	}
	reportContiguousProbe(50, 100, "blk00000.dat")
	st, ok = ContiguousReconcileProgress()
	if !ok || st.Phase != "probe" || st.Percent < 30 || st.Percent > 40 {
		t.Fatalf("probe mid: %#v", st)
	}
	if st.Detail == "" || st.Detail[:9] != "Verifying" {
		t.Fatalf("detail should say Verifying, got %q", st.Detail)
	}
	reportContiguousProbeDone()
	reportContiguousMeasure(5000, 0, 10000)
	st, ok = ContiguousReconcileProgress()
	if !ok || st.Phase != "measure" || st.Percent < 80 || st.Percent > 90 {
		t.Fatalf("measure mid: %#v", st)
	}
}

func TestContiguousReconcileProgressMonotonic(t *testing.T) {
	EndContiguousReconcile()
	BeginContiguousReconcile()
	defer EndContiguousReconcile()
	reportContiguousProbeDone()
	reportContiguousMeasure(8000, 0, 10000)
	mid, _ := ContiguousReconcileProgress()
	// A lower height must not pull the bar backward (old height%10000 creep bug).
	reportContiguousMeasure(1000, 0, 10000)
	st, _ := ContiguousReconcileProgress()
	if st.Percent+0.001 < mid.Percent {
		t.Fatalf("percent went backwards: mid=%v later=%v", mid.Percent, st.Percent)
	}
}
