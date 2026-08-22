// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyPerFileBinsMovesOutOfHotDir(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	payload, id := TestMinimalBlock()
	rootPath := filepath.Join(raw.Dir(), hex.EncodeToString(id[:])+".bin")
	if err := os.WriteFile(rootPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if !raw.HasLegacyPerFileBodies() {
		t.Fatal("expected leftover root *.bin")
	}
	n := raw.MigrateLegacyPerFileBinsNow()
	if n != 1 {
		t.Fatalf("migrated %d want 1", n)
	}
	if _, err := os.Stat(rootPath); !os.IsNotExist(err) {
		t.Fatalf("root bin should be gone: %v", err)
	}
	legacyPath := filepath.Join(raw.Dir(), legacyPerFileSubdir, hex.EncodeToString(id[:])+".bin")
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatal(err)
	}
	got, err := raw.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatal("legacy path must remain readable after migrate")
	}
	if !raw.HasStoredBody(id, 80) {
		t.Fatal("HasStoredBody should find legacy/")
	}
	if !raw.HasLegacyPerFileBodies() {
		t.Fatal("legacy/ still counts as leftover bodies")
	}
}
