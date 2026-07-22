// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// TestCrashKillBeforeRawPutSweepConvergence runs autoRecoverSweep after simulated kill-before-rename.
func TestCrashKillBeforeRawPutSweepConvergence(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, gen[:])
	genID := pow.BlockHashLE(genesisRaw[:80])

	store.SetAbortBeforeRawPutRenameForTest(true)
	if err := raw.Put(genID, genesisRaw); err == nil {
		t.Fatal("expected abort before rename")
	}
	tmpPath := filepath.Join(raw.Dir(), hex.EncodeToString(genID[:])+".bin.tmp")
	if _, err := os.Stat(tmpPath); err != nil {
		t.Fatal("expected .tmp after simulated kill")
	}

	bs := NewBlockStoreCtx(j, nil, p, raw, nil, nil)
	if _, err := autoRecoverSweep(chainDir, j, nil, p, bs, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("sweep should purge stale .tmp")
	}
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	if NeedsGenesisBlock(bs) {
		t.Fatal("genesis should be stored after sweep convergence")
	}
}
