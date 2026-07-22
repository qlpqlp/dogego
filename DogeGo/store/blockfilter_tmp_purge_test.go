// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPurgeStaleBlockFilterTemps(t *testing.T) {
	dir := t.TempDir()
	fx, err := OpenBlockFilterIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := filepath.Join(fx.Dir(), "010000000000000000000000000000000000000000000000000000000000000000.dat.tmp")
	if err := os.WriteFile(tmpPath, []byte{0x00, 0x01, 0x02}, 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := fx.PurgeStaleBlockFilterTemps()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("purged %d want 1", n)
	}
}
