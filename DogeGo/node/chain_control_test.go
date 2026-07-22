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

func TestInvalidateBlockTruncates(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	jpath := filepath.Join(dir, "headers.bin")
	j, err := store.OpenHeaderJournal(jpath, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	gen := g80
	h1 := append([]byte(nil), gen[:]...)
	prev := pow.BlockHashLE(gen[:])
	copy(h1[4:36], prev[:])
	h1[76] = 1
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	policy, err := store.LoadChainPolicy(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, nil, nil, nil)
	bs.Policy = policy
	display := pow.BlockHashHex(h1)
	if err := InvalidateBlock(j, nil, policy, bs, display); err != nil {
		t.Fatal(err)
	}
	tip, _ := j.TipHeight()
	if tip != 0 {
		t.Fatalf("tip %d", tip)
	}
	if !policy.IsInvalid(display) {
		t.Fatal("not invalid")
	}
}
