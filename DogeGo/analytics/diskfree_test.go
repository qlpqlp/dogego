// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVolumeUsageTempDir(t *testing.T) {
	dir := t.TempDir()
	free, total, err := VolumeUsage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if total == 0 {
		t.Fatal("expected non-zero total")
	}
	if free > total {
		t.Fatalf("free %d > total %d", free, total)
	}
	// Nested path should resolve to the same volume.
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	free2, total2, err := VolumeUsage(nested)
	if err != nil {
		t.Fatal(err)
	}
	if total2 != total {
		t.Fatalf("nested total %d want %d", total2, total)
	}
	_ = free2
}
