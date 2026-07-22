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

func TestListDescriptorsShMultiCLTVFromRedeem(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := LoadOrCreate(t.TempDir()+"/wallet.json", p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	ms := buildTestRedeem2of2(k1, k2)
	redeem := consensus.BuildCLTVMultisigRedeemScript(100_000, ms)
	want, ok := consensus.P2SHRedeemDescriptor(redeem, p.PubkeyHashAddrID)
	if !ok {
		t.Fatal("descriptor from CLTV multisig redeem")
	}
	h := chain.Hash160(redeem)
	var h160 [20]byte
	copy(h160[:], h)
	p2sh := chain.P2SHScriptFromScriptHash(h160)
	if err := w.AddWatchScript(p2sh); err != nil {
		t.Fatal(err)
	}
	if err := w.SetWatchRedeem(p2sh, redeem); err != nil {
		t.Fatal(err)
	}
	rows := w.ListDescriptors(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	found := false
	for _, r := range rows {
		if r.Desc == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing %q in %#v", want, rows)
	}
}
