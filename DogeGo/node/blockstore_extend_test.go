// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestStoreValidatedBlockRejectsNonTipExtend(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x11
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	blockRaw := store.MakeTestBlockRaw(t, g80[:]) // parent is genesis, not tip h1
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	want := pow.BlockHashLE(blockRaw[:80])
	err = bs.StoreValidatedBlock(want, blockRaw)
	if err == nil || (!strings.Contains(err.Error(), "does not extend") && !strings.Contains(err.Error(), "bad prev")) {
		t.Fatalf("want non-tip parent error, got %v", err)
	}
	if tip, _ := j.TipHeight(); tip != 1 {
		t.Fatalf("journal tip=%d want 1", tip)
	}
}

func TestStoreValidatedBlockExtendFailsValidation(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x22
	blockRaw := store.MakeTestBlockRaw(t, h1)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	want := pow.BlockHashLE(blockRaw[:80])
	err = bs.StoreValidatedBlock(want, blockRaw)
	if err == nil {
		t.Fatal("expected validation failure")
	}
	if !strings.Contains(err.Error(), "header extend") && !strings.Contains(err.Error(), "pow") && !strings.Contains(err.Error(), "nBits") {
		t.Fatalf("unexpected error: %v", err)
	}
	if tip, _ := j.TipHeight(); tip != 0 {
		t.Fatalf("journal should stay at genesis, tip=%d", tip)
	}
}
