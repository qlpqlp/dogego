// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store_test

import (
	"testing"

	"dogego/mempool"
	"dogego/store"
)

func TestBlockPutSidebandOnPut(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.SetBlockPutSideband(&store.BlockPutSideband{Journal: j, Pool: mempool.New(10)})
	if err := rs.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
}
