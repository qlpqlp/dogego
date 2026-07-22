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

func TestIsWitnessEnabled_alwaysFalse(t *testing.T) {
	if IsWitnessEnabled(5_000_000, chain.MainnetDogecoin) {
		t.Fatal("Dogecoin Core disables segwit witness rules")
	}
}
