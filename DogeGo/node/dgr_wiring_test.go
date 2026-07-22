// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestRelaySeedHostP2PAddrs(t *testing.T) {
	t.Parallel()
	got := relaySeedHostP2PAddrs([]string{
		"qlplock.ddns.net:24433",
		"203.0.113.10:24433",
		"relay.example.com",
		"",
		"qlplock.ddns.net:24433",
	}, 44556)
	want := []string{"qlplock.ddns.net:44556", "203.0.113.10:44556", "relay.example.com:44556"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
