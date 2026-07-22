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

func TestUpgradeLegacyTxIndexBatch(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	blockRaw := makeTestBlockRaw(t, g80[:])
	blockHashLE := pow.BlockHashLE(blockRaw[:80])
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(blockHashLE, blockRaw); err != nil {
		t.Fatal(err)
	}
	ix, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	txid := txidFromTestBlock(blockRaw)
	legacy := encodeTxIndexEntry(blockHashLE, 0, nil)
	path := filepath.Join(ix.RootDir(), txid)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	leg, v2, err := ix.FormatStats()
	if err != nil {
		t.Fatal(err)
	}
	if leg != 1 || v2 != 0 {
		t.Fatalf("before upgrade leg=%d v2=%d", leg, v2)
	}
	n, rem, err := UpgradeLegacyTxIndexBatch(dir, 16)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || rem != 0 {
		t.Fatalf("upgraded=%d remaining=%d", n, rem)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) <= txIndexMetaLen {
		t.Fatalf("still legacy len %d", len(data))
	}
}
