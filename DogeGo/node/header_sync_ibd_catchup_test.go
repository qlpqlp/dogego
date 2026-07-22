// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestShouldContinueHeaderCatchUpAt2000VsMainnetPeer(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	// Simulate one 2000-header batch landed (tip height 2000).
	appendFakeHeaderChain(t, j, gen[:80], 2000)

	if !shouldContinueHeaderCatchUpDuringIBD(j, 6_221_339) {
		t.Fatal("tip 2000 must continue catch-up when a mainnet peer reports millions of headers")
	}
	if shouldContinueHeaderCatchUpDuringIBD(j, 2000) {
		t.Fatal("tip 2000 should not continue when peer height is also 2000")
	}
}

func TestShouldContinueHeaderCatchUpUnknownPeerHeight(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(t.TempDir()+"/headers.bin", gen[:])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, gen[:80], 534_000)
	if shouldContinueHeaderCatchUpDuringIBD(j, 0) {
		t.Fatal("unknown peer height must not latch header catch-up when local tip is high")
	}
}

func TestNetworkPeerStartHeight(t *testing.T) {
	h := NetworkPeerStartHeight(&wire.DecodedVersion{StartHeight: 2000}, nil)
	if h != 2000 {
		t.Fatalf("got %d want 2000", h)
	}
	pm := &PeerMgr{sessions: map[string]*peerLink{
		"a": {peer: &wire.DecodedVersion{StartHeight: 6_000_000}},
	}}
	h = NetworkPeerStartHeight(&wire.DecodedVersion{StartHeight: 2000}, pm)
	if h != 6_000_000 {
		t.Fatalf("got %d want max across peers", h)
	}
}
