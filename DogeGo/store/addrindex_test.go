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

	"dogego/pow"
)

func TestReindexTxBuildsAddrIndex(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	blockRaw := makeTestBlockRaw(t, g80[:])
	id := pow.BlockHashLE(blockRaw[:80])
	if err := raw.Put(id, blockRaw); err != nil {
		t.Fatal(err)
	}
	rep, err := ReindexTxFromRawBlocks(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.BlocksIndexed != 1 || rep.TxFiles < 1 {
		t.Fatalf("rep %#v", rep)
	}
	addrRoot := filepath.Join(dir, "indexes", "addr", "recv")
	if _, err := os.Stat(addrRoot); err != nil {
		t.Fatalf("addr recv dir: %v", err)
	}
}
