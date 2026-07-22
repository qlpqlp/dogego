// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestAnnotateCmpctRelayMetrics(t *testing.T) {
	cmpctMetrics.In.Store(2)
	cmpctMetrics.ReconstructOK.Store(1)
	defer func() {
		cmpctMetrics.In.Store(0)
		cmpctMetrics.ReconstructOK.Store(0)
	}()
	out := map[string]any{}
	annotateCmpctHBCounts(out, nil, false, false)
	if out["dogego_cmpct_in"] != uint64(2) {
		t.Fatalf("in: %#v", out["dogego_cmpct_in"])
	}
	if out["dogego_cmpct_reconstruct_ok"] != uint64(1) {
		t.Fatalf("reconstruct_ok: %#v", out["dogego_cmpct_reconstruct_ok"])
	}
}
