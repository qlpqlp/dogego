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

func TestIndexBasicBlockFilterGenesis(t *testing.T) {
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
	if !fx.Has(id) {
		t.Fatal("filter not stored")
	}
	got, _, err := fx.Get(id)
	if err != nil || len(got) == 0 {
		t.Fatal(err)
	}
}
