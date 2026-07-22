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

func TestComputeBlockVersionAuxpowEra(t *testing.T) {
	j := &vbJournal{headers: []header80{{version: vbVersion(true), time: 1_500_000_000}}}
	ver := ComputeBlockVersion(j, chain.MainnetDogecoin, 1_000_000)
	if ver&(1<<8) == 0 {
		t.Fatal("expected auxpow bit")
	}
	if chainIDFromVersion(ver) != 0x62 {
		t.Fatalf("chain id %d want 98", chainIDFromVersion(ver))
	}
}

func TestBlockBaseVersion(t *testing.T) {
	if BlockBaseVersion(0x00620102) != 2 {
		t.Fatalf("base version got %d", BlockBaseVersion(0x00620102))
	}
}
