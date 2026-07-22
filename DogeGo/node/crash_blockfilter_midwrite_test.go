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

// TestCrashBlockFilterMidWriteRecovery simulates a torn filters/basic/*.dat.tmp after power-loss
// and expects autoRecoverSweep to purge it without rewinding headers.
func TestCrashBlockFilterMidWriteRecovery(t *testing.T) {
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
	fx, err := store.OpenBlockFilterIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := filepath.Join(fx.Dir(), "010000000000000000000000000000000000000000000000000000000000000000.dat.tmp")
	if err := os.WriteFile(tmpPath, []byte{0xde, 0xad}, 0o600); err != nil {
		t.Fatal(err)
	}

	bs := NewBlockStoreCtx(j, nil, p, nil, nil, nil)
	rewound, sweepErr := autoRecoverSweep(dir, j, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("sweep: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect header rewind for stale block filter tmp only")
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("expected block filter .dat.tmp removed by autoRecoverSweep")
	}
}
