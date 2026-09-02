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

func TestLocatorJournalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	raw.EnableWriteBehind()
	payload, id := TestMinimalBlock()
	if err := raw.Put(id, payload); err != nil {
		t.Fatal(err)
	}
	_ = raw.Flush()
	loc, ok, err := raw.lookupBlockLocator(id)
	if err != nil || !ok {
		t.Fatalf("lookup after put: ok=%v err=%v", ok, err)
	}
	if loc.RecordLen == 0 {
		t.Fatal("empty record len")
	}
	jnl := filepath.Join(raw.Dir(), "loc", locatorJournalFileName)
	if _, err := os.Stat(jnl); err != nil {
		t.Fatalf("journal missing: %v", err)
	}
	hexPath := blockLocatorPath(raw.locatorRoot(), id)
	if _, err := os.Stat(hexPath); err == nil {
		t.Fatal("expected no per-hash locator file when journal is used")
	}
	_ = raw.Close()

	raw2, err := OpenRawBlockStoreWithOpts(dir, BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	defer raw2.Close()
	got, err := raw2.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("reload len %d want %d", len(got), len(payload))
	}
}
