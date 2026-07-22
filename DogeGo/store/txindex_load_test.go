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

func TestLoadIndexedTx_V2AndLegacy(t *testing.T) {
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
	if err := ix.IndexBlock(blockHashLE, blockRaw); err != nil {
		t.Fatal(err)
	}
	tx, err := LoadIndexedTx(ix, raw, txid)
	if err != nil || tx.Version != 1 {
		t.Fatalf("v2 load err=%v tx=%v", err, tx)
	}
	legacy := encodeTxIndexEntry(blockHashLE, 0, nil)
	path := filepath.Join(ix.RootDir(), txid)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	tx, err = LoadIndexedTx(ix, raw, txid)
	if err != nil || tx.Version != 1 {
		t.Fatalf("legacy load err=%v", err)
	}
}
