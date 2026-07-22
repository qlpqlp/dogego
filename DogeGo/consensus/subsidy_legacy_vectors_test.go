// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"math/big"
	"testing"

	"dogego/chain"
)

func TestLegacySubsidyBugHint(t *testing.T) {
	err := fmt.Errorf("block connect: bad-cb-amount: out %d subsidy %d fees 0", legacySubsidyBugOutKoinu, legacySubsidyBugWrongSubsidy)
	if h := LegacySubsidyBugHint(err); h == "" {
		t.Fatal("expected rebuild hint")
	}
}

func TestRebootTestnetBlock1SubsidyTailMainnet(t *testing.T) {
	genesis, err := chain.Hash256FromDisplayHex(chain.GenesisBlockHashHex)
	if err != nil {
		t.Fatal(err)
	}
	got := BlockSubsidy(1, genesis, chain.RebootTestnet)
	want := int64(10_000) * KoinuPerCoin
	if got != want {
		t.Fatalf("height 1 subsidy got %d want %d (tail mainnet)", got, want)
	}
}

func TestMainnetBlock2Subsidy(t *testing.T) {
	block1, err := chain.Hash256FromDisplayHex("82bc68038f6034c0596b6e313729793a887fded6e92a31fbdf70863f89d9bea2")
	if err != nil {
		t.Fatal(err)
	}
	got := BlockSubsidy(2, block1, chain.MainnetDogecoin)
	want := int64(729752) * KoinuPerCoin
	if got != want {
		t.Fatalf("height 2 subsidy got %d want %d", got, want)
	}
}

func TestLegacySubsidySumFirst100kMatchesCore(t *testing.T) {
	var prev [32]byte
	var sum int64
	for h := int64(0); h <= 100_000; h++ {
		s := BlockSubsidy(h, prev, chain.MainnetDogecoin)
		sum += s
		addLEUint256(&prev, s)
	}
	want := int64(54894174438) * KoinuPerCoin
	if sum != want {
		t.Fatalf("legacy subsidy sum 0..100k got %d want %d", sum, want)
	}
}

// addLEUint256 adds n to prev interpreted as little-endian uint256 (Core arith_uint256 += nSubsidy).
func addLEUint256(b *[32]byte, n int64) {
	if n == 0 {
		return
	}
	acc := new(big.Int).SetBytes(le32ToBE(b[:]))
	acc.Add(acc, big.NewInt(n))
	out := acc.Bytes()
	clear(b[:])
	for i := 0; i < len(out) && i < 32; i++ {
		b[i] = out[len(out)-1-i]
	}
}

func le32ToBE(le []byte) []byte {
	be := make([]byte, len(le))
	for i := range le {
		be[len(le)-1-i] = le[i]
	}
	return be
}
