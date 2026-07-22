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
	"dogego/wire"
)

func TestDecodeTxIndexEntry_LegacyAndV2(t *testing.T) {
	var block [32]byte
	block[0] = 0xab
	legacy := encodeTxIndexEntry(block, 2, nil)
	if len(legacy) != txIndexMetaLen {
		t.Fatalf("legacy len %d", len(legacy))
	}
	hit, err := decodeTxIndexEntry(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if hit.BlockHashLE != block || hit.TxIndex != 2 || hit.TxRaw != nil {
		t.Fatalf("legacy hit %#v", hit)
	}
	raw := []byte{1, 0, 0, 0, 1, 2, 3}
	v2 := encodeTxIndexEntry(block, 0, raw)
	hit, err = decodeTxIndexEntry(v2)
	if err != nil {
		t.Fatal(err)
	}
	if string(hit.TxRaw) != string(raw) {
		t.Fatalf("v2 raw %x", hit.TxRaw)
	}
}

func TestTxIndex_IndexBlockWritesV2(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	blockRaw := makeTestBlockRaw(t, g80[:])
	blockHashLE := pow.BlockHashLE(blockRaw[:80])
	ix, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(blockHashLE, blockRaw); err != nil {
		t.Fatal(err)
	}
	txid := txidFromTestBlock(blockRaw)
	path := filepath.Join(ix.RootDir(), txid)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) <= txIndexMetaLen {
		t.Fatalf("expected v2 entry len %d", len(data))
	}
	hit, err := ix.LookupHit(txid)
	if err != nil {
		t.Fatal(err)
	}
	if len(hit.TxRaw) == 0 {
		t.Fatal("expected embedded tx raw")
	}
	tx, err := wire.DeserializeTx(hit.TxRaw)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Version != 1 {
		t.Fatalf("tx version %d", tx.Version)
	}
}
