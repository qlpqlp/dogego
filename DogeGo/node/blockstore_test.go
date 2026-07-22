// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestStoreValidatedBlockWireOnly(t *testing.T) {
	dir := t.TempDir()
	blockRaw, want := store.TestMinimalBlock()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(nil, nil, chain.Params{}, rs, nil, nil)
	if err := bs.StoreValidatedBlock(want, blockRaw); err != nil {
		t.Fatal(err)
	}
	if !rs.Has(want) {
		t.Fatal("expected block stored")
	}
}

func TestStoreValidatedBlockRejectsUnknownHeader(t *testing.T) {
	dir := t.TempDir()
	blockRaw, _ := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(dir+"/headers.bin", blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	params, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, params, rs, nil, nil)
	badPayload := append([]byte(nil), blockRaw...)
	badPayload[79] ^= 1
	badHash := pow.BlockHashLE(badPayload[:80])
	err = bs.StoreValidatedBlock(badHash, badPayload)
	if err == nil || (!strings.Contains(err.Error(), "header chain") && !strings.Contains(err.Error(), "journal")) {
		t.Fatalf("want header/journal error, got %v", err)
	}
}
