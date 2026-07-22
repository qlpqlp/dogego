// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"path/filepath"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestIBDExitLatch(t *testing.T) {
	ResetIBDExitLatchForTests()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Unix() - chain.DefaultMaxTipAge - 3600
	binary.LittleEndian.PutUint32(g80[68:72], uint32(stale))
	j, err := store.OpenHeaderJournal(filepath.Join(t.TempDir(), "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	ibdStale, _ := ibdProgress(j, "test", 0, 0, 0, chain.DefaultMaxTipAge, now, nil)
	if !ibdStale {
		t.Fatal("expected stale tip in IBD")
	}
	if !applyIBDExitLatch(ibdStale) {
		t.Fatal("still in IBD before latch")
	}
	// Leave IBD once (bodies caught up on testnet - no minimum work gate).
	if applyIBDExitLatch(false) {
		t.Fatal("expected left IBD")
	}
	ibdAgain, _ := ibdProgress(j, "test", 0, 0, 0, chain.DefaultMaxTipAge, now, nil)
	if !ibdAgain {
		t.Fatal("stale tip would be IBD without latch")
	}
	if applyIBDExitLatch(ibdAgain) {
		t.Fatal("latched: must not re-enter IBD")
	}
}
