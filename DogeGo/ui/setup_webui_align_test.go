// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

func TestAlignWebUIWithListen(t *testing.T) {
	got := alignWebUIWithListen("10.69.0.25:2013", "localhost:2013")
	if got != "10.69.0.25:2013" {
		t.Fatalf("loopback form default should follow pup listen, got %q", got)
	}
	got = alignWebUIWithListen("10.69.0.25:2013", "127.0.0.1:2013")
	if got != "10.69.0.25:2013" {
		t.Fatalf("127.0.0.1 form default should follow pup listen, got %q", got)
	}
	got = alignWebUIWithListen("10.69.0.25:2013", "10.69.0.25:2013")
	if got != "10.69.0.25:2013" {
		t.Fatalf("already aligned: got %q", got)
	}
	got = alignWebUIWithListen("127.0.0.1:2013", "localhost:2013")
	if got != "localhost:2013" {
		t.Fatalf("loopback listen should keep form webui, got %q", got)
	}
	got = alignWebUIWithListen("0.0.0.0:2013", "localhost:2013")
	if got != "0.0.0.0:2013" {
		t.Fatalf("all-interfaces listen should replace loopback webui, got %q", got)
	}
}
