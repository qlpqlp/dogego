// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"path/filepath"
	"testing"

	"dogego/chain"
)

func TestPeekChangeAddressDoesNotConsume(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	before := w.ChangeKeypoolSize()
	peek1 := w.PeekChangeAddress()
	peek2 := w.PeekChangeAddress()
	if peek1 == "" || peek1 != peek2 {
		t.Fatalf("peek %q %q", peek1, peek2)
	}
	if w.ChangeKeypoolSize() != before {
		t.Fatalf("pool %d want %d", w.ChangeKeypoolSize(), before)
	}
	if err := w.CommitChangeAddress(peek1); err != nil {
		t.Fatal(err)
	}
	if w.ChangeKeypoolSize() >= before {
		t.Fatalf("pool should shrink after commit")
	}
}
