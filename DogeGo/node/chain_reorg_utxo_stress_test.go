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

// TestTruncateChainReorgUtxoStress applies repeated operator truncates and checks UTXO hash_serialized
// matches a full replay reference at each kept tip (Milestone C).
func TestTruncateChainReorgUtxoStress(t *testing.T) {
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

	const tip = int64(10)
	prevID := genID
	for h := int64(1); h <= tip; h++ {
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
	if err := bs.RebuildUtxoThrough(tip); err != nil {
		t.Fatal(err)
	}

	wantAt := func(through int64) string {
		t.Helper()
		ref := store.NewUtxoCache()
		if err := ref.RebuildFromChain(j, rs, 0, through); err != nil {
			t.Fatal(err)
		}
		return ref.SerializedHashAtTip(j)
	}

	for keep := tip; keep >= 1; keep-- {
		if err := TruncateChainToHeight(j, nil, bs, keep); err != nil {
			t.Fatalf("truncate to %d: %v", keep, err)
		}
		want := wantAt(keep)
		if got := utxo.SerializedHashAtTip(j); got != want {
			t.Fatalf("after truncate %d: hash %s want %s", keep, got, want)
		}
		if utxo.TipHeight() != keep {
			t.Fatalf("utxo tip %d want %d", utxo.TipHeight(), keep)
		}
		rewound, err := autoRecoverSweep(dir, j, nil, p, bs, nil)
		if err != nil {
			t.Fatalf("sweep at %d: %v", keep, err)
		}
		if rewound {
			t.Fatalf("unexpected header rewind at truncate height %d", keep)
		}
	}
}
