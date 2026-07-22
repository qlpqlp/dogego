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

// TestCrashRawBlockMidWriteRecovery simulates power-loss during atomic raw block Put (leftover .bin.tmp)
// and verifies autoRecoverSweep removes stale temps without header rewind.
func TestCrashRawBlockMidWriteRecovery(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
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
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	genID := pow.BlockHashLE(genesisRaw[:80])
	tmpPath := filepath.Join(rs.Dir(), hex.EncodeToString(genID[:])+".bin.tmp")
	if err := os.WriteFile(tmpPath, genesisRaw, 0o600); err != nil {
		t.Fatal(err)
	}

	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	n, err := rs.PurgeStaleRawBlockTemps()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("purged %d stale tmp(s), want 1", n)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("expected .tmp removed")
	}

	rewound, sweepErr := autoRecoverSweep(dir, j, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("sweep: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect header rewind for stale raw tmp only")
	}
}
