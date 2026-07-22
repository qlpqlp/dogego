// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestIncrementalRelayFeePerKB(t *testing.T) {
	orig := IncrementalRelayFeePerKB()
	t.Cleanup(func() { SetIncrementalRelayFeePerKB(orig) })

	SetIncrementalRelayFeePerKB(50_000)
	if IncrementalRelayFeePerKB() != 50_000 {
		t.Fatalf("got %d", IncrementalRelayFeePerKB())
	}
	SetIncrementalRelayFeePerKB(0)
	if IncrementalRelayFeePerKB() != 50_000 {
		t.Fatal("zero should not change configured fee")
	}
}
