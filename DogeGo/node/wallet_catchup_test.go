// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"dogego/chain"
	"dogego/rpc"
	"dogego/store"
	"dogego/wallet"
)

func testWalletCatchUpSetup(t *testing.T) (context.Context, *rpc.DataPaths, *store.HeaderJournal, *store.RawBlockStore, *wallet.Disk, int64) {
	t.Helper()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, raw := putBundledTestChain(t, chainDir, 4)
	cont, err := store.ContiguousRawBodyHeight(j, raw)
	if err != nil || cont < 0 {
		t.Fatalf("contiguous=%d err=%v", cont, err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(chainDir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(cont)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	paths := &rpc.DataPaths{Utxo: utxo}
	return ctx, paths, j, raw, w, cont
}

func TestWalletCatchUpRescanSkipsWhenIndexedThroughTip(t *testing.T) {
	ctx, paths, j, raw, w, cont := testWalletCatchUpSetup(t)
	w.SeedScannedTx([]wallet.ScannedTx{{
		TxID: "aa", Category: "receive", BlockHeight: cont,
	}})
	var called atomic.Bool
	paths.SyncUtxo = func() error {
		called.Store(true)
		return nil
	}
	paths.WalletRescanBlocks = func(start int64) error {
		t.Fatalf("rescan from %d should be skipped when indexed through %d", start, cont)
		return nil
	}
	StartWalletCatchUpRescan(ctx, paths, j, raw, w)
	time.Sleep(250 * time.Millisecond)
	if called.Load() {
		t.Fatal("SyncUtxo should be skipped when UTXO tip covers contiguous bodies")
	}
}

func TestWalletCatchUpRescanIncrementalFromCursor(t *testing.T) {
	ctx, paths, j, raw, w, cont := testWalletCatchUpSetup(t)
	done := make(chan int64, 1)
	paths.WalletRescanBlocks = func(start int64) error {
		done <- start
		return nil
	}
	StartWalletCatchUpRescan(ctx, paths, j, raw, w)
	var start int64
	select {
	case start = <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for catch-up rescan (contiguous=%d max=%d)", cont, w.MaxScannedBlockHeight())
	}
	if start != 0 {
		t.Fatalf("rescan start=%d want 0 (fresh wallet, contiguous %d)", start, cont)
	}
}

func TestWalletCatchUpRescanSkipsDuringBodyIBD(t *testing.T) {
	ctx, paths, j, raw, w, cont := testWalletCatchUpSetup(t)
	prev, err := j.ReadHeaderAt(cont)
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, prev, 600)
	paths.WalletRescanBlocks = func(start int64) error {
		t.Fatalf("rescan from %d must not run while bodies lag headers", start)
		return nil
	}
	StartWalletCatchUpRescan(ctx, paths, j, raw, w)
	time.Sleep(250 * time.Millisecond)
}

func TestWalletCatchUpRescanContinuesAfterPartialIndex(t *testing.T) {
	ctx, paths, j, raw, w, cont := testWalletCatchUpSetup(t)
	if cont < 1 {
		t.Skipf("contiguous=%d; need at least height 1 for partial cursor", cont)
	}
	w.SeedScannedTx([]wallet.ScannedTx{{
		TxID: "bb", Category: "receive", BlockHeight: 0,
	}})
	done := make(chan int64, 1)
	paths.WalletRescanBlocks = func(start int64) error {
		done <- start
		return nil
	}
	StartWalletCatchUpRescan(ctx, paths, j, raw, w)
	select {
	case start := <-done:
		if start != 1 {
			t.Fatalf("rescan start=%d want 1 (indexed through 0, contiguous %d)", start, cont)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for partial catch-up rescan")
	}
}
