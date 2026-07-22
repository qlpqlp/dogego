// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "testing"

func TestEffectiveMaxTipAge(t *testing.T) {
	if EffectiveMaxTipAge(0) != DefaultMaxTipAge {
		t.Fatalf("zero got %d", EffectiveMaxTipAge(0))
	}
	if EffectiveMaxTipAge(-1) != DefaultMaxTipAge {
		t.Fatalf("negative got %d", EffectiveMaxTipAge(-1))
	}
	if EffectiveMaxTipAge(120) != 120 {
		t.Fatalf("custom got %d", EffectiveMaxTipAge(120))
	}
}
