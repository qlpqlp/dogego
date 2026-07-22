// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestReconcileGenesisWithContiguousShrinksStaleCache(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)

	// Simulate stale contiguous cache after genesis body was purged beneath replay progress.
	bs.contiguousMu.Lock()
	bs.contiguousTip = 100
	bs.contiguousMu.Unlock()

	if !NeedsGenesisBlock(bs) {
		t.Fatal("expected genesis missing with empty raw store")
	}
	ReconcileGenesisWithContiguous(bs)
	if NeedsGenesisBlock(bs) {
		t.Fatal("genesis should be restored from chainparams")
	}
	if got := bs.ContiguousRawHeight(); got != 0 {
		t.Fatalf("contiguous=%d want 0 after reconcile", got)
	}
}

func TestReconcileGenesisDuringReplayPreservesContiguous(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, utxo)

	bs.contiguousMu.Lock()
	bs.contiguousTip = 50
	bs.contiguousMu.Unlock()

	if !NeedsGenesisBlock(bs) {
		t.Fatal("expected genesis missing with empty raw store")
	}
	ReconcileGenesisWithContiguous(bs)
	if NeedsGenesisBlock(bs) {
		t.Fatal("genesis should be restored from chainparams")
	}
	if got := bs.ContiguousRawHeight(); got != 50 {
		t.Fatalf("contiguous=%d want 50 preserved during replay reconcile", got)
	}
}
