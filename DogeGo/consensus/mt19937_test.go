// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestGenerateMTRandomMainnetBlock2(t *testing.T) {
	// prev block 1 display seed chars [7:14] from 82bc68038f6034...
	seed := uint(0x38f6034)
	got := generateMTRandom(seed, 999999)
	want := 729751 // Core block 2 coinbase = 729752 DOGE = (1+rand)*COIN
	if got != want {
		rng := newMT19937(uint32(seed))
		t.Fatalf("generateMTRandom=%d want %d; first Uint32=%d", got, want, rng.Uint32())
	}
}

func TestGenerateMTRandomUsesBoostUniformIntDivision(t *testing.T) {
	// boost::mt19937(0x38f6034) first draw 3133550383; uniform_int(1,999999) => 729751.
	if got := generateMTRandom(0x38f6034, 999999); got != 729751 {
		t.Fatalf("got %d want 729751", got)
	}
}
