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

// TestCrashHeadersMidWriteRecovery simulates a torn headers.bin tail during sync and verifies
// reopen repair plus autoRecoverSweep converges without manual intervention (Milestone B).
func TestCrashHeadersMidWriteRecovery(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(chainDir, "headers.bin"), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	prev := append([]byte(nil), gen[:]...)
	for h := int64(1); h <= 5; h++ {
		h80 := append([]byte(nil), prev...)
		ph := pow.BlockHashLE(prev)
		copy(h80[4:36], ph[:])
		h80[76] ^= byte(h)
		binaryLETime(h80, 1_702_000_000+uint32(h)*60)
		if err := j.AppendHeaders([][]byte{h80}); err != nil {
			t.Fatal(err)
		}
		prev = append([]byte(nil), h80...)
	}
	preTip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.OpenFile(j.Path(), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 23)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	j2, err := store.OpenHeaderJournal(j.Path(), gen[:])
	if err != nil {
		t.Fatal(err)
	}
	postTip, err := j2.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	if postTip != preTip {
		t.Fatalf("reopen repair tip %d want %d", postTip, preTip)
	}

	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j2, nil, p, raw, nil, nil)
	rewound, sweepErr := autoRecoverSweep(chainDir, j2, nil, p, bs, nil)
	if sweepErr != nil {
		t.Fatalf("sweep after mid-write repair: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect rewind after tail-only corruption")
	}
}
