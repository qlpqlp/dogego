// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/store"
)

func TestOnContiguousAdvanceIndexFiltersSkipsIBDBackfill(t *testing.T) {
	dir := t.TempDir()
	blockRaw, _ := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err := j.AppendHeaders([][]byte{blockRaw[:80]}); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, params, raw, nil, nil)
	bs.contiguousTip = 5
	last := int64(-1)
	onContiguousAdvanceIndexFilters(bs, &last, 5, j, raw, nil, nil)
	if last != 5 {
		t.Fatalf("lastFilter=%d want 5 (skip [0..5] backfill during body IBD)", last)
	}
}

func TestUtxoIBDSyncNoteContiguous(t *testing.T) {
	u := newUtxoIBDSync("")
	u.noteContiguous(10)
	u.noteContiguous(8)
	if u.lastSyncedCont != 10 {
		t.Fatalf("lastSyncedCont=%d want 10", u.lastSyncedCont)
	}
}

func TestUtxoIBDSyncDefersSyncWhenConnectLag(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 32; i++ {
		if err := j.AppendHeaders([][]byte{blockRaw[:80]}); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(0)
	bs := NewBlockStoreCtx(j, nil, params, raw, nil, utxo)
	bs.contiguousTip = 200
	u := newUtxoIBDSync("")
	u.onContiguousAdvance(bs, utxo)
	if u.lastSyncedCont < 0 {
		t.Fatal("expected noteContiguous during connect lag")
	}
	if utxo.TipHeight() != 0 {
		t.Fatalf("utxo tip=%d want 0 (sync deferred to catch-up worker)", utxo.TipHeight())
	}
	if lag := ConnectCatchUpLag(bs, utxo); lag < connectCatchUpMinLag {
		t.Fatalf("lag=%d want >= %d", lag, connectCatchUpMinLag)
	}
}
