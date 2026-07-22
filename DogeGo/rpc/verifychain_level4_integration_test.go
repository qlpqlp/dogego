// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// TestExecVerifyChainLevel4OnStoredChain runs Core-shaped verifychain level 4 over native rawblocks.
func TestExecVerifyChainLevel4OnStoredChain(t *testing.T) {
	dir := t.TempDir()
	genesisRaw, err := chain.RebootTestnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
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

	const tip = int64(0)

	utxo := store.NewUtxoCache()
	if err := utxo.RebuildFromChain(j, rs, 0, tip); err != nil {
		t.Fatal(err)
	}

	p1, _ := json.Marshal(4)
	p2, _ := json.Marshal(0)
	res, code, msg := execVerifyChain("testnet", j, nil, rs, ix, nil, utxo, []json.RawMessage{p1, p2})
	if code != 0 || msg != "" {
		t.Fatalf("verifychain level 4: code=%d msg=%q", code, msg)
	}
	if res != true {
		t.Fatalf("verifychain level 4: result %#v", res)
	}
}
