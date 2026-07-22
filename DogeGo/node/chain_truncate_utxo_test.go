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

// TestTruncateChainToHeightRebuildUtxo verifies operator-style header rewind restores UTXO
// hash_serialized matching a linear sync through the same truncated tip.
func TestTruncateChainToHeightRebuildUtxo(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.EnableTxIndexing(ix, true)
	genID := pow.BlockHashLE(genesisRaw[:80])
	if err := rs.Put(genID, genesisRaw); err != nil {
		t.Fatal(err)
	}

	const targetHeight = int64(4)
	prevID := genID
	for h := int64(1); h <= targetHeight; h++ {
		prev, _ := j.ReadHeaderAt(h - 1)
		hdr := append([]byte(nil), prev...)
		copy(hdr[4:36], prevID[:])
		hdr[76] ^= byte(h)
		body := store.MakeTestBlockRaw(t, hdr)
		stored := append([]byte(nil), body[:80]...)
		id := pow.BlockHashLE(stored)
		if err := j.AppendHeaders([][]byte{stored}); err != nil {
			t.Fatal(err)
		}
		if err := rs.Put(id, body); err != nil {
			t.Fatal(err)
		}
		prevID = id
	}

	utxo := store.NewUtxoCache()
	bs := NewBlockStoreCtx(j, nil, p, rs, ix, utxo)
	if err := bs.RebuildUtxoThrough(targetHeight); err != nil {
		t.Fatal(err)
	}
	if err := TruncateChainToHeight(j, nil, bs, 2); err != nil {
		t.Fatal(err)
	}
	tip, err := j.TipHeight()
	if err != nil || tip != 2 {
		t.Fatalf("journal tip after truncate: %d err=%v", tip, err)
	}
	if utxo.TipHeight() != 2 {
		t.Fatalf("utxo tip after truncate rebuild: %d want 2", utxo.TipHeight())
	}
	ref := store.NewUtxoCache()
	if err := ref.RebuildFromChain(j, rs, 0, 2); err != nil {
		t.Fatal(err)
	}
	wantAt2 := ref.SerializedHashAtTip(j)
	if got := utxo.SerializedHashAtTip(j); got != wantAt2 {
		t.Fatalf("truncated utxo hash %s want %s", got, wantAt2)
	}
}
