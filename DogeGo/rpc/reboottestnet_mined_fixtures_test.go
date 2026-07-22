// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/chain"
)

func TestRebootTestnetMinedFixturesPresent(t *testing.T) {
	for h := 1; h <= 3; h++ {
		payload, err := RebootTestnetMinedFixture(h)
		if err != nil {
			t.Fatal(err)
		}
		if len(payload) < 80 {
			t.Fatalf("block%d len=%d", h, len(payload))
		}
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	if p.RelaxedPoW {
		t.Fatal("fixtures require real PoW reboot testnet")
	}
}
