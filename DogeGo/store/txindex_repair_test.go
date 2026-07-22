// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"

	"dogego/pow"
)

func TestLowestMissingBlockHeight(t *testing.T) {
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
	if err := raw.Put(pow.BlockHashLE(genesisRaw[:80]), genesisRaw); err != nil {
		t.Fatal(err)
	}
	low, err := LowestMissingBlockHeight(j, raw, 0, 0, 0)
	if err != nil || low >= 0 {
		t.Fatalf("low=%d err=%v", low, err)
	}
	h1 := append([]byte(nil), g80[:]...)
	gen := pow.BlockHashLE(g80[:])
	copy(h1[4:36], gen[:])
	h1[76] ^= 1
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	low, err = LowestMissingBlockHeight(j, raw, 1, 1, 0)
	if err != nil || low != 1 {
		t.Fatalf("low=%d err=%v", low, err)
	}
	ch, err := ContiguousRawBodyHeight(j, raw)
	if err != nil || ch != 0 {
		t.Fatalf("contiguous=%d err=%v", ch, err)
	}
}
