// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestParseP2PModeDefaults(t *testing.T) {
	s, err := ParseP2PMode("", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.Mode != P2PModeBoth || !s.Listen || s.MaxOutbound != defaultMaxOutbound || s.MaxInbound != defaultMaxInbound {
		t.Fatalf("defaults: %+v", s)
	}
}

func TestParseP2PModeCGNAT(t *testing.T) {
	s, err := ParseP2PMode("cgnat", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if s.Listen || s.MaxInbound != 0 {
		t.Fatalf("cgnat should not listen: %+v", s)
	}
}

func TestParseP2PModeInvalid(t *testing.T) {
	if _, err := ParseP2PMode("starlink-only", 0, 0); err == nil {
		t.Fatal("expected error")
	}
}
