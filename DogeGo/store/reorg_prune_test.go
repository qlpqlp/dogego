// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"

	"dogego/pow"
	"dogego/wire"
)

func TestPruneChainDataAboveHeight(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	blockRaw := makeTestBlockRaw(t, g80[:])
	h1 := blockRaw[:80]
	gen := pow.BlockHashLE(g80[:])
	copy(h1[4:36], gen[:])
	h1[76] ^= 1
	blockRaw = makeTestBlockRaw(t, h1)
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	txIx, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	id1 := pow.BlockHashLE(h1)
	if err := raw.Put(id1, blockRaw); err != nil {
		t.Fatal(err)
	}
	if err := txIx.IndexBlock(id1, blockRaw); err != nil {
		t.Fatal(err)
	}
	n, err := PruneChainDataAboveHeight(j, raw, txIx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("removed %d", n)
	}
	if raw.Has(id1) {
		t.Fatal("raw block still present")
	}
	if _, _, err := txIx.Lookup(txidFromTestBlock(blockRaw)); err == nil {
		t.Fatal("tx index entry still present")
	}
}

func txidFromTestBlock(blockRaw []byte) string {
	pb, err := wire.ParseBlock(blockRaw)
	if err != nil {
		return ""
	}
	if len(pb.Txs) == 0 {
		return ""
	}
	h := pb.Txs[0].TxHash()
	return txidRPCFileName(h)
}
