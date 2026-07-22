// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// TestCrashActiveHeaderSegment_MainnetFieldHeaders verifies segment tail repair with real mainnet genesis + field headers 1-3.
func TestCrashActiveHeaderSegment_MainnetFieldHeaders(t *testing.T) {
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderChain(chainDir, gen[:80])
	if err != nil {
		t.Fatal(err)
	}
	var wantTip3 string
	for _, spec := range mainnetCanonicalBlockSpecs {
		if spec.Height < 1 || spec.Height > 3 {
			continue
		}
		raw, err := buildMainnetCanonicalBlockRaw(spec)
		if err != nil {
			t.Fatalf("height %d: %v", spec.Height, err)
		}
		if err := j.AppendHeaders([][]byte{raw[:80]}); err != nil {
			t.Fatal(err)
		}
		if spec.Height == 3 {
			wantTip3 = spec.WantHash
		}
	}
	preTip, err := j.TipHeight()
	if err != nil || preTip != 3 {
		t.Fatalf("tip before crash=%d err=%v", preTip, err)
	}

	segPath := filepath.Join(chainDir, "headers", "seg", "0000000000.bin")
	f, err := os.OpenFile(segPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 41)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(segPath+".tmp", []byte("stale segment tmp"), 0o600); err != nil {
		t.Fatal(err)
	}

	j2, err := store.OpenHeaderChain(chainDir, gen[:80])
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
	h80, err := j2.ReadHeaderAt(3)
	if err != nil {
		t.Fatal(err)
	}
	if got := pow.BlockHashHex(h80); got != wantTip3 {
		t.Fatalf("height 3 hash %s want %s", got, wantTip3)
	}
}
