// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"math/big"
)

// marginalReorgWorkDivisor: incoming fork must beat displaced work by at least 1/divisor (Core-style avoid hair-trigger reorgs).
const marginalReorgWorkDivisor = 20 // 5%

func marginalChainWorkThreshold(displaced *big.Int) *big.Int {
	if displaced == nil || displaced.Sign() <= 0 {
		return big.NewInt(1)
	}
	m := new(big.Int).Div(displaced, big.NewInt(marginalReorgWorkDivisor))
	if m.Sign() == 0 {
		return big.NewInt(1)
	}
	return m
}

// shouldDeferMarginalReorg reports whether a fork with higher but tiny chain-work advantage should wait for more headers.
func shouldDeferMarginalReorg(incoming, displaced *big.Int, precious bool) bool {
	if precious || incoming == nil || displaced == nil || incoming.Cmp(displaced) <= 0 {
		return false
	}
	delta := new(big.Int).Sub(incoming, displaced)
	return delta.Cmp(marginalChainWorkThreshold(displaced)) < 0
}

func marginalReorgErr(incoming, displaced *big.Int) error {
	delta := new(big.Int).Sub(incoming, displaced)
	return fmt.Errorf("headers: fork deferred (marginal chain work +%s; need >%s to reorg)",
		delta.String(), marginalChainWorkThreshold(displaced).String())
}
