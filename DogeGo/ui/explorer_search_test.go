// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

func TestNetworkFromUISlug(t *testing.T) {
	for _, s := range []string{"testnet", "Mainnet", "MAINNET"} {
		if _, err := networkFromUISlug(s); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
	}
	if _, err := networkFromUISlug("foo"); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsAllDecimalDigits(t *testing.T) {
	if !isAllDecimalDigits("12345") {
		t.Fatal()
	}
	if isAllDecimalDigits("12a") {
		t.Fatal()
	}
	if isAllDecimalDigits("") {
		t.Fatal()
	}
}

func TestIs64Hex(t *testing.T) {
	s := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if !is64Hex(s) {
		t.Fatal()
	}
	if is64Hex(s[:63]) {
		t.Fatal()
	}
	if is64Hex("gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg") {
		t.Fatal()
	}
}
