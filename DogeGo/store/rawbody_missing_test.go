// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/pow"
)

func TestLowestMissingSearchStartAndFrom(t *testing.T) {
	dir := t.TempDir()
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := MakeTestBlockRaw(t, g80[:])
	j, err := OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rbDir := filepath.Join(dir, "rawblocks")
	if err := os.MkdirAll(rbDir, 0o700); err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(genesisRaw[:80])
	if err := os.WriteFile(filepath.Join(rbDir, hex.EncodeToString(genHash[:])+".bin"), genesisRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	h1 := append([]byte(nil), genesisRaw[:80]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 1
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	searchStart := LowestMissingSearchStart(j, raw, 0, 0)
	if searchStart != 1 {
		t.Fatalf("search start %d want 1", searchStart)
	}
	low, err := LowestMissingBlockHeightFrom(j, raw, searchStart, 1, 0)
	if err != nil || low != 1 {
		t.Fatalf("low=%d err=%v", low, err)
	}
}
