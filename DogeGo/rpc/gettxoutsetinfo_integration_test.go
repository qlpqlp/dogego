// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// TestExecGetTxOutSetInfoMatchesUtxoCache ensures gettxoutsetinfo hash_serialized matches the UTXO cache at tip.
func TestExecGetTxOutSetInfoMatchesUtxoCache(t *testing.T) {
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
	genID := pow.BlockHashLE(genesisRaw[:80])
	if err := rs.Put(genID, genesisRaw); err != nil {
		t.Fatal(err)
	}

	utxo := store.NewUtxoCache()
	if err := utxo.RebuildFromChain(j, rs, 0, 0); err != nil {
		t.Fatal(err)
	}
	wantHash := utxo.SerializedHashAtTip(j)

	info, code, msg := execGetTxOutSetInfo(j, rs, utxo, nil, nil)
	if code != 0 || msg != "" {
		t.Fatalf("gettxoutsetinfo: code=%d msg=%q", code, msg)
	}
	got, _ := info["hash_serialized"].(string)
	if got != wantHash {
		t.Fatalf("hash_serialized %s want %s", got, wantHash)
	}
	if info["height"].(int64) != 0 {
		t.Fatalf("height %#v", info["height"])
	}
	if info["txouts"].(int64) != 1 {
		t.Fatalf("txouts %#v", info["txouts"])
	}
}
