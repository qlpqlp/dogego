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
	"dogego/rpc"
	"dogego/store"
)

// TestCrashActivePutRestartConvergence simulates kill-during-write for raw Put and header segment
// append, then runs the same startup path as the live node (reopen + autoRecoverSweep).
func TestCrashActivePutRestartConvergence(t *testing.T) {
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
	for h := int64(1); h <= 3; h++ {
		if _, err := rpc.AppendRebootTestnetMinedFixture(j, int(h)); err != nil {
			t.Fatal(err)
		}
	}
	tipBefore, _ := j.TipHeight()

	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, gen[:])
	genID := pow.BlockHashLE(genesisRaw[:80])
	// Phase 1: kill after rename with truncated .bin
	if err := raw.Put(genID, genesisRaw); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(raw.Dir(), hex.EncodeToString(genID[:])+".bin")
	if err := os.Truncate(binPath, 100); err != nil {
		t.Fatal(err)
	}
	// Phase 2: kill mid-Put on another height (complete .tmp only)
	h1Raw := store.MakeTestBlockRaw(t, mustHeaderAt(t, j, 1))
	h1ID := pow.BlockHashLE(h1Raw[:80])
	if err := os.WriteFile(filepath.Join(raw.Dir(), hex.EncodeToString(h1ID[:])+".bin.tmp"), h1Raw, 0o600); err != nil {
		t.Fatal(err)
	}
	// Phase 3: torn segment tail + stale segment .tmp
	segPath := filepath.Join(chainDir, "headers", "seg", "0000000000.bin")
	f, err := os.OpenFile(segPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 23)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()
	_ = os.WriteFile(segPath+".tmp", []byte("orphan"), 0o600)
	_ = os.WriteFile(filepath.Join(chainDir, "headers", "manifest.json.tmp"), []byte("{}"), 0o600)

	// Restart path
	j, err = store.OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	tipAfterOpen, _ := j.TipHeight()
	if tipAfterOpen > tipBefore {
		t.Fatalf("open repair must not advance tip beyond committed state: %d > %d", tipAfterOpen, tipBefore)
	}

	raw, err = store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, nil, nil)
	rewound, sweepErr := autoRecoverSweep(chainDir, j, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("sweep: %v", sweepErr)
	}
	if rewound && tipAfterOpen == tipBefore {
		t.Fatal("unexpected header rewind when only torn tail / stale temps present")
	}
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatalf("EnsureLocalGenesis after crash recovery: %v", err)
	}
	if NeedsGenesisBlock(bs) {
		t.Fatal("genesis should be restored after convergence sweep")
	}
	if _, err := os.Stat(segPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("segment .tmp should be gone after restart")
	}
}

func mustHeaderAt(t *testing.T, j *store.HeaderJournal, height int64) []byte {
	t.Helper()
	h, err := j.ReadHeaderAt(height)
	if err != nil {
		t.Fatal(err)
	}
	return h
}
