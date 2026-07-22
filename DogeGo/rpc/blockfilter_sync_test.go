// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/store"
)

func TestBlockFilterSyncedHeight(t *testing.T) {
	rawBlk, id := store.TestMinimalBlock()
	dir := t.TempDir()
	raw, _ := store.OpenRawBlockStore(dir)
	ix, _ := store.OpenTxIndex(dir)
	fx, _ := store.OpenBlockFilterIndex(dir)
	_ = raw.Put(id, rawBlk)
	_ = ix.IndexBlock(id, rawBlk)
	if err := IndexBasicBlockFilter(fx, id, rawBlk, nil, raw, ix); err != nil {
		t.Fatal(err)
	}
	h80 := rawBlk[:80]
	j := &memJournal{tip: 0, count: 1, hdrs: [][]byte{append([]byte(nil), h80...)}}
	if BlockFilterSyncedHeight(j, 0, fx) != 0 {
		t.Fatal("expected height 0 synced")
	}
	if BlockFilterSyncedHeight(j, 0, nil) != -1 {
		t.Fatal("nil index")
	}
}
