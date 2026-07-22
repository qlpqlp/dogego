// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestHeaderIBDProgress(t *testing.T) {
	if p := HeaderIBDProgress(9840, 5_900_000); p <= 0 || p > 0.01 {
		t.Fatalf("early header progress got %v want small positive", p)
	}
	if HeaderIBDProgress(5_900_000, 5_900_000) != 1 {
		t.Fatal("caught up to peer")
	}
}

func TestEffectiveIBDDisplayProgress_headersFirst(t *testing.T) {
	p := EffectiveIBDDisplayProgress(9840, -1, 5_900_000, true)
	if p < 0.001 {
		t.Fatalf("want header-based progress, got %v", p)
	}
}

func TestEffectiveIBDDisplayProgress_bodyWhenHeadersFarAhead(t *testing.T) {
	want := BodyVerificationProgress(534_000, 616)
	p := EffectiveIBDDisplayProgress(534_000, 616, 6_000_000, true)
	if p != want {
		t.Fatalf("got %v want body progress %v (not header %% during deep body IBD)", p, want)
	}
}
