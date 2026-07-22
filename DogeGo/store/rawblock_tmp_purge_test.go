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

func TestPurgeStaleRawBlockTemps(t *testing.T) {
	dir := t.TempDir()
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := MakeTestBlockRaw(t, g80[:])
	id := pow.BlockHashLE(genesisRaw[:80])
	tmpPath := filepath.Join(raw.Dir(), hex.EncodeToString(id[:])+".bin.tmp")
	if err := os.WriteFile(tmpPath, genesisRaw[:120], 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := raw.PurgeStaleRawBlockTemps()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("purged %d want 1", n)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("expected .tmp removed")
	}
}
