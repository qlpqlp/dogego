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

// TestCrashHeaderAuxTornTailSweepRecovery verifies autoRecoverSweep repairs a force-killed headers_aux.bin tail.
func TestCrashHeaderAuxTornTailSweepRecovery(t *testing.T) {
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
	for h := int64(1); h <= 3; h++ {
		h80 := make([]byte, 80)
		copy(h80, gen[:])
		binaryLETime(h80, 1_702_000_000+uint32(h)*60)
		if err := j.AppendHeaders([][]byte{h80}); err != nil {
			t.Fatal(err)
		}
	}
	auxPath := filepath.Join(chainDir, "headers_aux.bin")
	aux, err := store.OpenHeaderAuxJournal(auxPath, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := aux.EnsureRecordCount(4); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(auxPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0xff, 0xff, 0xff, 0xff}); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, nil, nil)
	if _, err := autoRecoverSweep(chainDir, j, nil, p, bs, nil); err != nil {
		t.Fatal(err)
	}
	aux2, err := store.OpenHeaderAuxJournal(auxPath, 4)
	if err != nil {
		t.Fatal(err)
	}
	if aux2.RecordCount() != 4 {
		t.Fatalf("aux count after sweep=%d want 4", aux2.RecordCount())
	}
}
