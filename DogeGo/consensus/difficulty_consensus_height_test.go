// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/chain"
)

// TestLookupConsensusParentHeightAt145000 verifies era parameters follow block height (Core GetConsensus(pindex)).
func TestLookupConsensusParentHeightAt145000(t *testing.T) {
	parent := LookupConsensus(chain.MainnetDogecoin, 144999)
	child := LookupConsensus(chain.MainnetDogecoin, 145000)
	if parent.PowTargetTimespan != 4*60*60 {
		t.Fatalf("pre-145000 timespan %d want 14400", parent.PowTargetTimespan)
	}
	if child.PowTargetTimespan != 60 {
		t.Fatalf("post-145000 timespan %d want 60", child.PowTargetTimespan)
	}
	if parent.Digishield {
		t.Fatal("pre-145000 should not use Digishield modulation")
	}
	if !child.Digishield {
		t.Fatal("post-145000 should use Digishield")
	}
}
