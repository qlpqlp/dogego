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

func TestSyncUtxoCacheBoundedLimitsSteps(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := j.AppendHeaders([][]byte{blockRaw[:80]}); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := raw.Put(hash, blockRaw); err != nil {
			t.Fatal(err)
		}
	}
	txIx, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	bs := NewBlockStoreCtx(j, nil, params, raw, txIx, utxo)
	bs.contiguousTip = 7
	if err := bs.SyncUtxoCacheBounded(2); err != nil {
		t.Fatal(err)
	}
	if got := utxo.TipHeight(); got > 1 {
		t.Fatalf("bounded sync tip=%d want <=1", got)
	}
}
