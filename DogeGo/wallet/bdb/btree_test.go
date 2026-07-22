// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package bdb

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestIsBDBFileMetaMagic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.dat")
	page := make([]byte, 512)
	binary.LittleEndian.PutUint32(page[12:16], btreeMagic)
	binary.LittleEndian.PutUint32(page[20:24], 512)
	page[28] = btreeMeta
	if err := os.WriteFile(path, page, 0o600); err != nil {
		t.Fatal(err)
	}
	if !IsBDBFile(path) {
		t.Fatal("expected bdb file")
	}
}

func TestOpenKVRejectsTruncated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.dat")
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenKV(path); err == nil {
		t.Fatal("expected error")
	}
}
