// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package p2p

import "testing"

func TestPreferIPv4First(t *testing.T) {
	in := []string{"[2001:db8::1]:22556", "1.2.3.4:22556", "[::1]:22556", "5.6.7.8:22556"}
	got := PreferIPv4First(in)
	if !HostPortIsIPv4(got[0]) || !HostPortIsIPv4(got[1]) {
		t.Fatalf("ipv4 first: %v", got)
	}
	if HostPortIsIPv4(got[2]) {
		t.Fatalf("ipv6 should follow: %v", got)
	}
}
