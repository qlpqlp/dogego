// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
	"dogego/consensus"
)

func TestListDescriptorsBareMulti(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(t.TempDir()+"/wallet.json", p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	ms := buildTestRedeem2of2(k1, k2)
	want, ok := consensus.MultiDescriptorFromRedeem(ms)
	if !ok {
		t.Fatal("descriptor from redeem")
	}
	if err := w.AddWatchScript(ms); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range w.ListDescriptors(p.PubkeyHashAddrID, p.ScriptHashAddrID) {
		if r.Desc == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing %q", want)
	}
}
