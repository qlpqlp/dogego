// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"math/big"
	"testing"
)

func TestShouldDeferMarginalReorg(t *testing.T) {
	cur := big.NewInt(1000)
	inc := big.NewInt(1030) // +3% - below 5% threshold
	if !shouldDeferMarginalReorg(inc, cur, false) {
		t.Fatal("expected defer for marginal advantage")
	}
	inc2 := big.NewInt(1100) // +10%
	if shouldDeferMarginalReorg(inc2, cur, false) {
		t.Fatal("should accept clear advantage")
	}
	if shouldDeferMarginalReorg(inc, cur, true) {
		t.Fatal("precious should not defer")
	}
}
