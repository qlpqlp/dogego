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

func TestImportDescriptorsShMultiRejectsUnknownKey(t *testing.T) {
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	k3, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "sh(multi(2," + h1 + "," + h2 + "))"
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif3, _ := chain.EncodeWIF(k3.Serialize(), p.PrivKeyWIFVersion, true)
	addr := w.DefaultAddress()
	paths := &DataPaths{
		WalletAddress:        func() string { return addr },
		WalletDefaultAddress: func() string { return addr },
		WalletImportPrivKey: func(s string) error {
			return w.ImportPrivKey(s, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
		},
		WalletImportWatch:    func(script []byte) error { return w.AddWatchScript(script) },
		WalletSetWatchRedeem: w.SetWatchRedeem,
	}
	req, _ := json.Marshal(map[string]interface{}{
		"desc": desc,
		"keys": []string{wif3},
	})
	out, code, msg := execImportDescriptors("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(req) + "]")})
	if code != 0 {
		t.Fatalf("importdescriptors: %s", msg)
	}
	row := out.([]interface{})[0].(map[string]interface{})
	if row["success"] == true {
		t.Fatal("expected failure for key not in descriptor")
	}
	errObj := row["error"].(map[string]interface{})
	if errObj["message"] != "importdescriptors: private key is not included in the descriptor" {
		t.Fatalf("%#v", errObj)
	}
}
