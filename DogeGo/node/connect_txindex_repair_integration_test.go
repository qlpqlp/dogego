// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/consensus"
	"dogego/store"
	"dogego/wire"
)

// TestMaybeRepairTxIndexOnConnectErrRestoresFundingHeight verifies the connect-error repair hook
// rebuilds sparse indexes/tx entries (mainnet connect stall class at ~6856).
func TestMaybeRepairTxIndexOnConnectErrRestoresFundingHeight(t *testing.T) {
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
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, ix, store.NewUtxoCache())
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	bs.Utxo.SetTipHeightForTest(0)
	bs.SeedContiguousTip(0)
	var fundHash [32]byte
	if err := wire.ForEachBlockTx(blockRaw, func(_ uint32, tx *wire.Tx) error {
		fundHash = tx.TxHash()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(ix.RootDir(), store.TxIDIndexFileNameLE(fundHash)))
	if _, ok, _ := consensus.FundingTxHeight(ix, j, fundHash); ok {
		t.Fatal("expected sparse index before repair hook")
	}
	maybeRepairTxIndexOnConnectErr(bs, fmt.Errorf("input 0: missing funding height"))
	h, ok, err := consensus.FundingTxHeight(ix, j, fundHash)
	if err != nil || !ok || h != 0 {
		t.Fatalf("after connect-error repair height=%d ok=%v err=%v want 0 true nil", h, ok, err)
	}
}

func TestMaybeRepairTxIndexOnConnectStallRunsSparseRepair(t *testing.T) {
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
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, ix, store.NewUtxoCache())
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	bs.Utxo.SetTipHeightForTest(0)
	bs.SeedContiguousTip(0)
	var fundHash [32]byte
	_ = wire.ForEachBlockTx(blockRaw, func(_ uint32, tx *wire.Tx) error {
		fundHash = tx.TxHash()
		return nil
	})
	_ = os.Remove(filepath.Join(ix.RootDir(), store.TxIDIndexFileNameLE(fundHash)))
	sparse, err := store.TxIndexSparseThrough(j, raw, ix, p.Net, 0, 1)
	if err != nil || !sparse {
		t.Fatalf("want sparse before stall repair sparse=%v err=%v", sparse, err)
	}
	stallErr := fmt.Errorf("utxo sync: connect stalled at height 0 (contiguous bodies through 0)")
	maybeRepairTxIndexOnConnectStall(bs, stallErr)
	sparse, err = store.TxIndexSparseThrough(j, raw, ix, p.Net, 0, 1)
	if err != nil || sparse {
		t.Fatalf("want index complete after stall repair sparse=%v err=%v", sparse, err)
	}
}
