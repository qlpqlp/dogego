// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
	"dogego/consensus"
	"dogego/wallet"
)

func TestGetAddressInfoCLTVMultisigSolvable(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	ms := buildTestRedeem2of2(k1, k2)
	redeem := consensus.BuildCLTVMultisigRedeemScript(100_000, ms)
	h := scriptHash160(redeem)
	p2sh := chain.P2SHScriptFromScriptHash(h)
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	_ = w.AddWatchScript(p2sh)
	_ = w.SetWatchRedeem(p2sh, redeem)
	wif1, _ := chain.EncodeWIF(k1.Serialize(), p.PrivKeyWIFVersion, true)
	_ = w.ImportPrivKey(wif1, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
	addr := chain.ScriptPubKeyAddress(p2sh, p.PubkeyHashAddrID, p.ScriptHashAddrID)
	paths := importDescTestPaths(w, p)
	info, code, msg := execGetAddressInfo("test", paths, []json.RawMessage{json.RawMessage(`"` + addr + `"`)})
	if code != 0 {
		t.Fatalf("getaddressinfo: %s", msg)
	}
	if info.(map[string]interface{})["solvable"] != true {
		t.Fatalf("solvable want true")
	}
}

func buildTestRedeem2of2(k1, k2 *secp256k1.PrivateKey) []byte {
	pub1 := k1.PubKey().SerializeCompressed()
	pub2 := k2.PubKey().SerializeCompressed()
	var b []byte
	b = append(b, 0x51)
	b = append(b, byte(len(pub1)))
	b = append(b, pub1...)
	b = append(b, byte(len(pub2)))
	b = append(b, pub2...)
	b = append(b, 0x52, 0xae)
	return b
}
