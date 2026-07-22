// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow

import "testing"

func TestTargetHexFromCompact(t *testing.T) {
	// Mainnet-like compact 0x1e0ffff0
	h := TargetHexFromCompact(0x1e0ffff0)
	if len(h) != 64 {
		t.Fatalf("len %d", len(h))
	}
	if BitsHex(0x1e0ffff0) != "1e0ffff0" {
		t.Fatal("bits hex")
	}
}
