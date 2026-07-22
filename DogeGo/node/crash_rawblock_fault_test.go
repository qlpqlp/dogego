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

// TestAutoRecoverPurgeUndersizedRawStubAfterCrash simulates a torn/undersized raw block file
// and expects automatic purge during recovery sweep (Milestone B scaffold).
func TestAutoRecoverPurgeUndersizedRawStubAfterCrash(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, g80[:])
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), genesisRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	prev, _ := j.ReadHeaderAt(0)
	h1 := append([]byte(nil), prev...)
	ph := pow.BlockHashLE(prev)
	copy(h1[4:36], ph[:])
	h1[76] ^= 0x55
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}

	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	genID := pow.BlockHashLE(genesisRaw[:80])
	if err := rs.Put(genID, genesisRaw); err != nil {
		t.Fatal(err)
	}
	id1 := pow.BlockHashLE(h1)
	stubPath := filepath.Join(dir, "rawblocks", hex.EncodeToString(id1[:])+".bin")
	if err := os.WriteFile(stubPath, []byte{0x01, 0x02, 0x03}, 0o600); err != nil {
		t.Fatal(err)
	}

	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	n, err := bs.PurgeInadequateRawBodies()
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected undersized raw stub to be purged")
	}
	if rs.Has(id1) {
		t.Fatal("stub should be removed after purge")
	}

	rewound, sweepErr := autoRecoverSweep(dir, j, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("sweep after stub purge: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect header rewind for raw stub only")
	}
}
