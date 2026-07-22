// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
	"dogego/wallet"
)

func TestGetAddressInfoMultisigWatchRedeem(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	nJ, _ := json.Marshal(1)
	keysJ, _ := json.Marshal([]string{h1, h2})
	m, code, msg := execCreateMultisig("test", []json.RawMessage{nJ, keysJ})
	if code != 0 {
		t.Fatalf("createmultisig: %s", msg)
	}
	msAddr := m["address"].(string)
	redeem, _ := hex.DecodeString(m["redeemScript"].(string))
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

	paths := walletTestPaths(w, p)
	info, code2, msg2 := execGetAddressInfo("test", paths, []json.RawMessage{json.RawMessage(`"` + msAddr + `"`)})
	if code2 != 0 {
		t.Fatalf("getaddressinfo: %s", msg2)
	}
	out := info.(map[string]interface{})
	if out["solvable"] != true {
		t.Fatalf("solvable want true got %#v", out)
	}
	if out["isscript"] != true {
		t.Fatalf("isscript %#v", out)
	}
	spk := out["scriptPubKey"].(map[string]interface{})
	if spk["dogego_script_template"] != "multisig" {
		t.Fatalf("scriptPubKey %#v", spk)
	}
	val, code3, _ := execValidateAddress("test", paths, []json.RawMessage{json.RawMessage(`"` + msAddr + `"`)})
	if code3 != 0 || val["isscript"] != true {
		t.Fatalf("validateaddress %#v", val)
	}
}

func walletTestPaths(w *wallet.Disk, p chain.Params) *DataPaths {
	return &DataPaths{
		WalletAddress:        func() string { return w.DefaultAddress() },
		WalletDefaultAddress: func() string { return w.DefaultAddress() },
		WalletContainsAddress: func(addr string) bool {
			return w.ContainsAddress(addr)
		},
		WalletIsWatchAddress: func(addr string) bool {
			return w.IsWatchAddress(addr, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		},
		WalletWatchRedeemScript: func(spk []byte) []byte {
			return w.WatchRedeemScript(spk)
		},
		WalletWIFForAddress: func(addr string) (string, error) {
			priv, err := w.PrivKeyForAddress(addr)
			if err != nil {
				return "", err
			}
			return chain.EncodeWIF(priv.Serialize(), p.PrivKeyWIFVersion, true)
		},
		WalletKnownAddresses: func() []string {
			return w.KnownAddresses(p.PubkeyHashAddrID, p.ScriptHashAddrID)
		},
		WalletWIFs: func() []string {
			wifs, _ := w.AllWIFs(p.PrivKeyWIFVersion)
			return wifs
		},
	}
}
