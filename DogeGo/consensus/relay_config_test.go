// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestApplyNodeRelayFeesIncrementalRaisesMin(t *testing.T) {
	origMin := MinRelayTxFeePerKB()
	origIncr := IncrementalRelayFeePerKB()
	t.Cleanup(func() {
		SetMinRelayTxFeePerKB(origMin)
		SetIncrementalRelayFeePerKB(origIncr)
	})

	ApplyNodeRelayFees(250_000, 0, false)
	if IncrementalRelayFeePerKB() != 250_000 {
		t.Fatalf("incr %d", IncrementalRelayFeePerKB())
	}
	if MinRelayTxFeePerKB() != 250_000 {
		t.Fatalf("min %d", MinRelayTxFeePerKB())
	}
}

func TestApplyNodeRelayFeesExplicitMin(t *testing.T) {
	origMin := MinRelayTxFeePerKB()
	origIncr := IncrementalRelayFeePerKB()
	t.Cleanup(func() {
		SetMinRelayTxFeePerKB(origMin)
		SetIncrementalRelayFeePerKB(origIncr)
	})

	ApplyNodeRelayFees(250_000, 150_000, true)
	if MinRelayTxFeePerKB() != 150_000 {
		t.Fatalf("min %d", MinRelayTxFeePerKB())
	}
}
