// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestSeedBlockAssistCandidatesFromFixedPeers(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.FixedPeers) == 0 {
		t.Skip("no fixed peers in chain params")
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderChain(dir, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, g80[:], 100)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	c := seedBlockAssistCandidates(context.Background(), p, bs, nil, nil, nil)
	if c == nil || c.Len() == 0 {
		t.Fatal("expected assist pool from fixed peers without DNS")
	}
}
