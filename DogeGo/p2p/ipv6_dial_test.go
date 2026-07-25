// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package p2p

import (
	"errors"
	"syscall"
	"testing"
)

func TestHostPortIsIPv6(t *testing.T) {
	if !HostPortIsIPv6("[2001:db8::1]:44556") {
		t.Fatal("expected ipv6")
	}
	if HostPortIsIPv6("1.2.3.4:44556") {
		t.Fatal("ipv4")
	}
	if HostPortIsIPv6("seed.example:44556") {
		t.Fatal("hostname")
	}
}

func TestObserveDialErrorDisablesIPv6(t *testing.T) {
	ResetIPv6DialGateForTest()
	defer ResetIPv6DialGateForTest()
	err := errors.New("dial tcp [2001:db8::1]:44556: connect: network is unreachable")
	if !ObserveDialError("[2001:db8::1]:44556", err) {
		t.Fatal("expected first disable")
	}
	if !IPv6DialsDisabled() {
		t.Fatal("flag")
	}
	if ObserveDialError("[2001:db8::2]:44556", err) {
		t.Fatal("second observe should not re-fire")
	}
	in := []string{"[2001:db8::1]:1", "1.2.3.4:1", "[::1]:1"}
	got := PreferIPv4First(in)
	if len(got) != 1 || got[0] != "1.2.3.4:1" {
		t.Fatalf("filtered: %v", got)
	}
}

func TestIsNetworkUnreachableErrno(t *testing.T) {
	if !IsNetworkUnreachable(syscall.ENETUNREACH) {
		t.Fatal("ENETUNREACH")
	}
}
