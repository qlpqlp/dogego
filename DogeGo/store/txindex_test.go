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

func TestTxIndex_LookupInvalidTxid(t *testing.T) {
	dir := t.TempDir()
	ix, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ix.Lookup("abcd")
	if err == nil {
		t.Fatal("expected error for short txid")
	}
	if !strings.Contains(err.Error(), "64") {
		t.Fatalf("unexpected err: %v", err)
	}
	_, _, err = ix.Lookup(strings.Repeat("g", 64))
	if err == nil {
		t.Fatal("expected error for non-hex")
	}
}

func TestTxIndexStats(t *testing.T) {
	dir := t.TempDir()
	ix, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	n, sz, err := ix.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || sz != 0 {
		t.Fatalf("empty got n=%d sz=%d", n, sz)
	}
	name := strings.Repeat("a", 64)
	path := filepath.Join(ix.RootDir(), name)
	if err := os.WriteFile(path, make([]byte, 36), 0o600); err != nil {
		t.Fatal(err)
	}
	ix.invalidateStatsCache()
	n, sz, err = ix.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || sz != 36 {
		t.Fatalf("got n=%d sz=%d", n, sz)
	}
}
