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

func TestBlockSubsidySimplified_post145k(t *testing.T) {
	const interval = int64(100_000)
	for height := int64(145_000); height < 600_000; height++ {
		got := BlockSubsidySimplified(height, interval)
		want := int64((500_000 * KoinuPerCoin) >> (height / interval))
		if got != want {
			t.Fatalf("height %d: got %d want %d", height, got, want)
		}
	}
	if g := BlockSubsidySimplified(600_000, interval); g != 10_000*KoinuPerCoin {
		t.Fatalf("600k: got %d", g)
	}
	if g := BlockSubsidySimplified(700_000, interval); g != 10_000*KoinuPerCoin {
		t.Fatalf("700k: got %d", g)
	}
}

func TestBlockSubsidyLegacyHeight0(t *testing.T) {
	got := BlockSubsidy(0, [32]byte{}, chain.MainnetDogecoin)
	if got < 2*KoinuPerCoin || got > 1_000_000*KoinuPerCoin {
		t.Fatalf("height 0 out of legacy range: %d", got)
	}
}

func TestBlockSubsidySimplifiedAt145k(t *testing.T) {
	got := BlockSubsidy(145_000, [32]byte{}, chain.MainnetDogecoin)
	want := int64(250_000) * KoinuPerCoin
	if got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestCoinbaseHeightFromScript(t *testing.T) {
	script := []byte{0x03, 0x39, 0x30, 0x00} // height 12345 LE
	h, ok := CoinbaseHeightFromScript(script)
	if !ok || h != 12345 {
		t.Fatalf("got %d ok=%v", h, ok)
	}
}
