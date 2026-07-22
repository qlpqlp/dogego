// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
)

// TestCrashKillHeaderSegmentSweepConvergence verifies autoRecoverSweep after stall-killed segment append.
func TestCrashKillHeaderSegmentSweepConvergence(t *testing.T) {
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
	store.StallAfterHeaderSegTmpWriteForTest(1 * time.Millisecond)
	// Stall is long enough for test thread; we simulate kill by leaving .tmp via abort pattern:
	store.StallAfterHeaderSegTmpWriteForTest(0)
	if _, err := rpc.AppendRebootTestnetMinedFixture(j, 1); err != nil {
		t.Fatal(err)
	}
	tipBefore, _ := j.TipHeight()
	if tipBefore != 1 {
		t.Fatalf("tip=%d want 1", tipBefore)
	}

	segPath := filepath.Join(chainDir, "headers", "seg", "0000000000.bin.tmp")
	// Simulate kill mid-next-append: corrupt by leaving orphan tmp from copied segment state.
	data, err := os.ReadFile(filepath.Join(chainDir, "headers", "seg", "0000000000.bin"))
	if err != nil {
		t.Fatal(err)
	}
	block2, err := rpc.RebootTestnetMinedFixture(2)
	if err != nil {
		t.Fatal(err)
	}
	h2 := append([]byte(nil), block2[:80]...)
	if err := os.WriteFile(segPath, append(data, h2...), 0o600); err != nil {
		t.Fatal(err)
	}

	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, nil, nil)
	if _, err := autoRecoverSweep(chainDir, j, nil, p, bs, nil); err != nil {
		t.Fatal(err)
	}
	j2, err := store.OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(segPath); !os.IsNotExist(err) {
		t.Fatal("expected orphan segment .tmp purged after reopen/sweep")
	}
	tipAfter, _ := j2.TipHeight()
	if tipAfter != tipBefore {
		t.Fatalf("tip after sweep=%d want %d", tipAfter, tipBefore)
	}
}
