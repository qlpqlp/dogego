// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestEnsureLocalGenesisMainnet(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
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
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	if !NeedsGenesisBlock(bs) {
		t.Fatal("expected genesis missing before EnsureLocalGenesis")
	}
	if err := EnsureLocalGenesis(bs); err != nil {
		t.Fatal(err)
	}
	if NeedsGenesisBlock(bs) {
		t.Fatal("genesis should be stored")
	}
	if bs.ContiguousRawHeight() != 0 {
		t.Fatalf("contiguous=%d want 0", bs.ContiguousRawHeight())
	}
}
