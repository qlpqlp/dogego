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

func TestTxVerificationProgressMainnet(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	now := uint32(time.Now().Unix())
	binary.LittleEndian.PutUint32(g80[68:72], now)
	j, err := store.OpenHeaderJournal(filepath.Join(t.TempDir(), "h.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	pv, ok := txVerificationProgress("main", j, nil, 0)
	if !ok {
		t.Fatal("expected mainnet tx verification curve")
	}
	if pv <= 0 || pv > 1 {
		t.Fatalf("progress %v", pv)
	}
}
