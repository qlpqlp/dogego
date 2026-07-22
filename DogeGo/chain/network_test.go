// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "testing"

func TestParseNetwork(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Network
	}{
		{"testnet", RebootTestnet},
		{"reboottestnet", RebootTestnet},
		{"mainnet", MainnetDogecoin},
		{"main", MainnetDogecoin},
	} {
		got, err := ParseNetwork(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.in, got, tc.want)
		}
	}
	if _, err := ParseNetwork("bogus"); err == nil {
		t.Fatal("expected error")
	}
}
