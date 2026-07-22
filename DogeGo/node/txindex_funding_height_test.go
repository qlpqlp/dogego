// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/consensus"
	"dogego/store"
	"dogego/wire"
)

// Core connect path uses FundingTxHeight during sequence-lock checks; sparse txindex must be repairable from rawblocks.
func TestRepairTxIndexRestoresFundingTxHeight(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	var fundHash [32]byte
	if err := wire.ForEachBlockTx(blockRaw, func(_ uint32, tx *wire.Tx) error {
		fundHash = tx.TxHash()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	h, ok, err := consensus.FundingTxHeight(ix, j, fundHash)
	if err != nil || !ok || h != 0 {
		t.Fatalf("before wipe height=%d ok=%v err=%v want 0 true nil", h, ok, err)
	}
	entries, err := os.ReadDir(ix.RootDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected tx index files")
	}
	if err := os.Remove(filepath.Join(ix.RootDir(), entries[0].Name())); err != nil {
		t.Fatal(err)
	}
	_, ok, err = consensus.FundingTxHeight(ix, j, fundHash)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected missing funding height after tx file removed")
	}
	rep, err := store.RepairTxIndexFromRaw(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rep.BlocksIndexed < 1 {
		t.Fatalf("repair blocks_indexed=%d", rep.BlocksIndexed)
	}
	h, ok, err = consensus.FundingTxHeight(ix, j, fundHash)
	if err != nil || !ok || h != 0 {
		t.Fatalf("after repair height=%d ok=%v err=%v want 0 true nil", h, ok, err)
	}
}
