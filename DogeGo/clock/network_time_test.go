// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package clock

import "testing"

func TestNetworkUnixUsesOffset(t *testing.T) {
	SetMockUnix(1_000_000)
	defer SetMockUnix(0)
	if got := NetworkUnix(120); got != 1_000_120 {
		t.Fatalf("got %d want 1000120", got)
	}
}
