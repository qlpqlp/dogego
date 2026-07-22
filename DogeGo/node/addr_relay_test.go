// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestIsRelayHostPort(t *testing.T) {
	if !IsRelayHostPort("relay.example.com:24433") {
		t.Fatal("dns relay target")
	}
	if !IsRelayHostPort("8.8.8.8:24433") {
		t.Fatal("ip relay target")
	}
	if IsRelayHostPort("127.0.0.1:24433") {
		t.Fatal("loopback relay rejected")
	}
	if IsRelayHostPort("relay.example.com:0") {
		t.Fatal("zero port rejected")
	}
}

func TestRelayAddrBookDNS(t *testing.T) {
	b := NewAddrBook()
	addr := "relay.example.com:24433"
	b.NoteRelayTry(addr)
	b.NoteRelaySuccess(addr)
	if !b.IsTried(addr) {
		t.Fatal("relay target should be tried")
	}
	if b.RelayDialScore(addr) <= 0 {
		t.Fatalf("score %d", b.RelayDialScore(addr))
	}
}
