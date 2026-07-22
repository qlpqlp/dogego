// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow_test

import (
	"testing"

	"dogego/chain"
	"dogego/pow"
)

func TestGenesisBlockHash(t *testing.T) {
	h, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	got := pow.BlockHashHex(h[:])
	if got != chain.GenesisBlockHashHex {
		t.Fatalf("block hash: got %s want %s", got, chain.GenesisBlockHashHex)
	}
}

func TestDifficultyFromCompact(t *testing.T) {
	d, err := pow.DifficultyFromCompact(0x1e0ffff0)
	if err != nil {
		t.Fatal(err)
	}
	if d <= 0 {
		t.Fatalf("unexpected difficulty %f", d)
	}
}
