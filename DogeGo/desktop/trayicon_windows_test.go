//go:build windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import "testing"

func TestTrayIconICOHeader(t *testing.T) {
	b := trayIconBytes()
	if len(b) < 22 {
		t.Fatalf("ico too short: %d", len(b))
	}
	if b[0] != 0 || b[1] != 0 || b[2] != 1 || b[3] != 0 {
		t.Fatalf("bad ico header % x", b[:4])
	}
	if b[4] != 1 || b[5] != 0 {
		t.Fatalf("expected one image, got %d", b[4])
	}
}
