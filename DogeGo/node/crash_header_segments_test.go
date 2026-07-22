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

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// TestCrashHeaderSegmentsMidWriteRecovery verifies segment tail repair, stale .tmp purge,
// and checkpoint realignment after a simulated force-kill during header append (Milestone B).
func TestCrashHeaderSegmentsMidWriteRecovery(t *testing.T) {
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
	prev := append([]byte(nil), gen[:]...)
	for h := int64(1); h <= 5; h++ {
		h80 := append([]byte(nil), prev...)
		ph := pow.BlockHashLE(prev)
		copy(h80[4:36], ph[:])
		h80[76] ^= byte(h)
		binaryLETime(h80, 1_702_000_000+uint32(h)*60)
		if err := j.AppendHeaders([][]byte{h80}); err != nil {
			t.Fatal(err)
		}
		prev = append([]byte(nil), h80...)
	}
	preTip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}

	segPath := filepath.Join(chainDir, "headers", "seg", "0000000000.bin")
	f, err := os.OpenFile(segPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 37)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(segPath+".tmp", []byte("stale tmp"), 0o600); err != nil {
		t.Fatal(err)
	}

	j2, err := store.OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	postTip, err := j2.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	if postTip != preTip {
		t.Fatalf("segment tail repair tip %d want %d", postTip, preTip)
	}
	if _, err := os.Stat(segPath + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("expected stale segment .tmp purged on open")
	}
	if _, err := j2.BestBlockHashHex(); err != nil {
		t.Fatalf("BestBlockHashHex after segment repair: %v", err)
	}

	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j2, nil, p, raw, nil, nil)
	rewound, sweepErr := autoRecoverSweep(chainDir, j2, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("sweep after segment tail repair: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect rewind after tail-only segment corruption")
	}
}
