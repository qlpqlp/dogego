// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// TestCrashIndexFilterTmpSweepRecovery verifies autoRecoverSweep purges stale tx/filter .tmp files.
func TestCrashIndexFilterTmpSweepRecovery(t *testing.T) {
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
	txIx, err := store.OpenTxIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx, err := store.OpenBlockFilterIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(txIx.RootDir(), strings.Repeat("b", 64)+".tmp"), []byte{0x01}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filterIx.Dir(), "010000000000000000000000000000000000000000000000000000000000000000.dat.tmp"), []byte{0x02}, 0o600); err != nil {
		t.Fatal(err)
	}

	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, txIx, nil)
	if _, err := autoRecoverSweep(chainDir, j, nil, p, bs, nil); err != nil {
		t.Fatal(err)
	}
	if matches, _ := filepath.Glob(filepath.Join(txIx.RootDir(), "*.tmp")); len(matches) != 0 {
		t.Fatal("expected tx index .tmp purged by sweep")
	}
	if matches, _ := filepath.Glob(filepath.Join(filterIx.Dir(), "*.dat.tmp")); len(matches) != 0 {
		t.Fatal("expected filter .tmp purged by sweep")
	}
}
