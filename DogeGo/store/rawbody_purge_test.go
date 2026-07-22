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

	"dogego/chain"
	"dogego/pow"
)

func TestPurgeInadequateRawBodiesRemovesMainnetStub(t *testing.T) {
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
	id := pow.BlockHashLE(genesisRaw[:80])
	stub := make([]byte, MainnetGenesisStubTestBytes)
	copy(stub[:80], genesisRaw[:80])
	path := filepath.Join(raw.Dir(), hex.EncodeToString(id[:])+".bin")
	if err := os.WriteFile(path, stub, 0o600); err != nil {
		t.Fatal(err)
	}
	n, _, err := PurgeInadequateRawBodies(j, raw, chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("purged %d want 1", n)
	}
	if raw.Has(id) {
		t.Fatal("stub should be removed")
	}
}
