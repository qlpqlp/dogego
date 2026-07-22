// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"testing"
	"time"

	"dogego/chain"
	"dogego/store"
)

func TestBlockFilterCatchUpWorkerIndexesDuringIBD(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		if err := j.AppendHeaders([][]byte{blockRaw[:80]}); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for h := int64(0); h <= 10; h++ {
		if err := raw.Put(hash, blockRaw); err != nil {
			t.Fatal(err)
		}
	}
	txIx, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx, err := store.OpenBlockFilterIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, params, raw, txIx, nil)
	bs.contiguousTip = 10
	last := int64(5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startBlockFilterCatchUpWorker(ctx, bs, j, raw, filterIx, txIx, &last)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if last >= 10 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if last < 10 {
		t.Fatalf("lastFilter=%d want >=10 after catch-up worker", last)
	}
}
