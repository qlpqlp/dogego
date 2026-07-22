// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"testing"

	"dogego/pow"
)

func TestRepairTxIndexIfLagSkipsWhenCountsMatch(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	blockRaw := makeTestBlockRaw(t, g80[:])
	id := pow.BlockHashLE(blockRaw[:80])
	if err := raw.Put(id, blockRaw); err != nil {
		t.Fatal(err)
	}
	txIx, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := txIx.IndexBlock(id, blockRaw); err != nil {
		t.Fatal(err)
	}
	_, ran, err := RepairTxIndexIfLag(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ran {
		t.Fatal("expected no repair when tx file count >= raw block count")
	}
}
