// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestFilterIndexThroughMonotonic(t *testing.T) {
	SetFilterIndexThrough(100)
	if got := FilterIndexThrough(); got != 100 {
		t.Fatalf("through=%d want 100", got)
	}
	SetFilterIndexThrough(50)
	if got := FilterIndexThrough(); got != 100 {
		t.Fatalf("through=%d want 100 after lower update", got)
	}
	SetFilterIndexThrough(200)
	if got := FilterIndexThrough(); got != 200 {
		t.Fatalf("through=%d want 200", got)
	}
}
