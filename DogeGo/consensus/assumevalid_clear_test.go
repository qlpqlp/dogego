// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestAssumeValidClearResolution(t *testing.T) {
	a := NewAssumeValid("mainnet", "")
	a.mu.Lock()
	a.height = 5_050_000
	a.mu.Unlock()
	a.ClearResolution()
	if a.Height() >= 0 {
		t.Fatalf("height %d want -1", a.Height())
	}
}
