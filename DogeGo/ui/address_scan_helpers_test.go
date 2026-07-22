// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/chain"
	"dogego/store"
)

func TestNormalizeScanAddressMainnet(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	var h20 [20]byte
	for i := range h20 {
		h20[i] = byte(i + 1)
	}
	canonical := chain.Base58CheckEncode(p.PubkeyHashAddrID, h20[:])
	got := normalizeScanAddress(canonical, p.PubkeyHashAddrID, p.ScriptHashAddrID)
	if got != canonical {
		t.Fatalf("normalize %q => %q want %q", canonical, got, canonical)
	}
}

func TestMergeUtxoOutputsForAddress(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	var h20 [20]byte
	copy(h20[:], []byte("addr-merge-test-01"))
	addr := chain.Base58CheckEncode(p.PubkeyHashAddrID, h20[:])
	spk := chain.P2PKHScriptFromPubKeyHash(h20)

	u := store.NewUtxoCache()
	var op [36]byte
	var h [32]byte
	h[0] = 1
	copy(op[:32], h[:])
	u.AddUtxoForTest(op, store.UtxoEntry{Value: 5e8, PkScript: spk, Height: 42})

	var hits []AddrTxHit
	var total int64
	mergeUtxoOutputsForAddress(&hits, func() *store.UtxoCache { return u }, addr, p.PubkeyHashAddrID, p.ScriptHashAddrID, &total)
	if len(hits) != 1 {
		t.Fatalf("hits=%d want 1", len(hits))
	}
	if hits[0].Height != 42 || hits[0].ValueKoinu != 5e8 {
		t.Fatalf("hit %#v", hits[0])
	}
	if total != 5e8 {
		t.Fatalf("total=%d", total)
	}
}
