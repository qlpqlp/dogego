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

// TestOperatorWorkflowStandaloneCertification exercises a Core-shaped operator path without P2P:
// sync bodies, auto-recovery sweep, header truncate, and UTXO rebuild convergence.
func TestOperatorWorkflowStandaloneCertification(t *testing.T) {
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

	const tip = int64(3)
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
	rewound, err := autoRecoverSweep(dir, j, nil, p, bs, nil)
	if err != nil {
		t.Fatalf("startup recovery sweep: %v", err)
	}
	if rewound {
		t.Fatal("unexpected header rewind on clean chain")
	}
	if utxo.TipHeight() != tip {
		t.Fatalf("utxo tip after sweep: %d want %d", utxo.TipHeight(), tip)
	}

	if err := TruncateChainToHeight(j, nil, bs, 1); err != nil {
		t.Fatal(err)
	}
	if th, _ := j.TipHeight(); th != 1 {
		t.Fatalf("journal tip after truncate: %d", th)
	}
	if utxo.TipHeight() != 1 {
		t.Fatalf("utxo tip after truncate: %d", utxo.TipHeight())
	}

	ref := store.NewUtxoCache()
	if err := ref.RebuildFromChain(j, rs, 0, 1); err != nil {
		t.Fatal(err)
	}
	wantTrunc := ref.SerializedHashAtTip(j)
	if got := utxo.SerializedHashAtTip(j); got != wantTrunc {
		t.Fatalf("truncated hash %s want %s", got, wantTrunc)
	}

	rewound, err = autoRecoverSweep(dir, j, nil, p, bs, nil)
	if err != nil {
		t.Fatalf("post-truncate recovery sweep: %v", err)
	}
	if rewound {
		t.Fatal("unexpected rewind after truncate")
	}
}
