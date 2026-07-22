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

	"dogego/chain"
	"dogego/wire"
)

func TestTxIndexSparseThroughDetectsMissingTxFile(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := TestMinimalBlock()
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	sparse, err := TxIndexSparseThrough(j, raw, ix, p.Net, 0, 1)
	if err != nil || sparse {
		t.Fatalf("full index: sparse=%v err=%v", sparse, err)
	}
	var fundHash [32]byte
	_ = wire.ForEachBlockTx(blockRaw, func(_ uint32, tx *wire.Tx) error {
		fundHash = tx.TxHash()
		return nil
	})
	path := filepath.Join(ix.RootDir(), txidRPCFileName(fundHash))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	sparse, err = TxIndexSparseThrough(j, raw, ix, p.Net, 0, 1)
	if err != nil || !sparse {
		t.Fatalf("after wipe want sparse=true, got sparse=%v err=%v", sparse, err)
	}
}

func TestRepairTxIndexIfSparseRebuilds(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := TestMinimalBlock()
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	ix, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	var fundHash [32]byte
	_ = wire.ForEachBlockTx(blockRaw, func(_ uint32, tx *wire.Tx) error {
		fundHash = tx.TxHash()
		return nil
	})
	_ = os.Remove(filepath.Join(ix.RootDir(), txidRPCFileName(fundHash)))
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	rep, ran, err := RepairTxIndexIfSparse(dir, j, raw, ix, p.Net, 0, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ran || rep.BlocksIndexed < 1 {
		t.Fatalf("repair ran=%v rep=%+v", ran, rep)
	}
	sparse, err := TxIndexSparseThrough(j, raw, ix, p.Net, 0, 1)
	if err != nil || sparse {
		t.Fatalf("after repair sparse=%v err=%v", sparse, err)
	}
}
