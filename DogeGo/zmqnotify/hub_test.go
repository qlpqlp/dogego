// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package zmqnotify

import "testing"

func TestNormalizeEndpoint(t *testing.T) {
	ep, err := normalizeEndpoint("127.0.0.1:28332")
	if err != nil || ep != "tcp://127.0.0.1:28332" {
		t.Fatalf("got %q err=%v", ep, err)
	}
	ep2, err := normalizeEndpoint("tcp://[::1]:28332")
	if err != nil || ep2 != "tcp://[::1]:28332" {
		t.Fatalf("got %q err=%v", ep2, err)
	}
}

func TestHash32Wire(t *testing.T) {
	var h [32]byte
	h[0] = 1
	h[31] = 0xff
	w := hash32Wire(h)
	if w[0] != 0xff || w[31] != 1 {
		t.Fatalf("wire order %x", w)
	}
}
