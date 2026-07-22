// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import (
	"math/big"
	"testing"
)

func TestMinimumChainWorkMainnet(t *testing.T) {
	min := MinimumChainWork(MainnetDogecoin)
	if min == nil || min.Sign() == 0 {
		t.Fatal("expected mainnet minimum chain work")
	}
	if MinimumChainWorkHex(MainnetDogecoin) != mainnetMinimumChainWorkHex {
		t.Fatal("hex mismatch")
	}
	w, ok := MinimumChainWorkForRPCChain("main")
	if !ok || w.Cmp(min) != 0 {
		t.Fatal("rpc main mapping")
	}
	if _, ok := MinimumChainWorkForRPCChain("test"); ok {
		t.Fatal("testnet should have no minimum")
	}
}

func TestMinimumChainWorkOrder(t *testing.T) {
	min := MinimumChainWork(MainnetDogecoin)
	small := big.NewInt(1)
	if small.Cmp(min) >= 0 {
		t.Fatal("genesis work should be below minimum")
	}
}
