// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPurgeStaleTxIndexTemps(t *testing.T) {
	dir := t.TempDir()
	ix, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := filepath.Join(ix.RootDir(), strings.Repeat("a", 64)+".tmp")
	if err := os.WriteFile(tmpPath, []byte{0x01}, 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := ix.PurgeStaleTxIndexTemps()
	if err != nil || n != 1 {
		t.Fatalf("purged %d err=%v", n, err)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("expected tmp removed")
	}
}
