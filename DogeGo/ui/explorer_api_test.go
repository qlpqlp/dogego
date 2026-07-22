// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/store"
)

func TestRawBlockStoreKeyDisplayHexFilename(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	payload, want := store.TestMinimalBlock()
	h80 := payload[:80]
	var alt [32]byte
	for i := 0; i < 32; i++ {
		alt[i] = want[31-i]
	}
	altPath := filepath.Join(dir, "rawblocks", hex.EncodeToString(alt[:])+".bin")
	if err := os.WriteFile(altPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	_ = want
	key, ok, used := rawBlockStoreKey(rs, h80)
	if !ok {
		t.Fatal("expected raw block found when only explorer-order filename exists")
	}
	if key != alt {
		t.Fatalf("key %x want alt %x", key, alt)
	}
	if !used {
		t.Fatal("expected usedDisplayName")
	}
}

func TestRawBlockStoreKeyWireFilenamePreferred(t *testing.T) {
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	payload, want := store.TestMinimalBlock()
	h80 := payload[:80]
	if err := rs.Put(want, payload); err != nil {
		t.Fatal(err)
	}
	key, ok, used := rawBlockStoreKey(rs, h80)
	if !ok {
		t.Fatal("expected found")
	}
	if key != want {
		t.Fatalf("key %x want %x", key, want)
	}
	if used {
		t.Fatal("expected canonical wire key first")
	}
}
