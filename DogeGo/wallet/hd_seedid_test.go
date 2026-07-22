// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"dogego/chain"
)

func TestHDSeedIDHex(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	id := w.HDSeedIDHex()
	if len(id) != 64 {
		t.Fatalf("hdseedid len %d", len(id))
	}
	w.mu.Lock()
	h := sha256.Sum256(w.hdSeed)
	w.mu.Unlock()
	if id != hex.EncodeToString(h[:]) {
		t.Fatalf("hdseedid mismatch")
	}
}
