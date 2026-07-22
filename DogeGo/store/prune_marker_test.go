// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "testing"

func TestPruneMarkerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := SavePruneMarker(dir, 100_000); err != nil {
		t.Fatal(err)
	}
	h, ok := LoadPruneMarker(dir)
	if !ok || h != 100_000 {
		t.Fatalf("got %d ok=%v", h, ok)
	}
	if _, ok := LoadPruneMarker(""); ok {
		t.Fatal("empty dir should not load")
	}
}
