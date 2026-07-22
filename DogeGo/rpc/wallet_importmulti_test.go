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
	"dogego/wallet"
)

func TestImportMultiPubkeysMultisig(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	pubkeys, _ := json.Marshal([]string{
		hexEncode(k1.PubKey().SerializeCompressed()),
		hexEncode(k2.PubKey().SerializeCompressed()),
	})
	req, _ := json.Marshal(map[string]interface{}{
		"pubkeys":  json.RawMessage(pubkeys),
		"required": 2,
		"keys":     []interface{}{},
	})
	dir := t.TempDir()
	w, _ := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	paths := &DataPaths{
		WalletAddress:     func() string { return w.Address() },
		WalletImportWatch: func(script []byte) error { return w.AddWatchScript(script) },
		WalletWatchScripts: func() [][]byte { return w.WatchScripts() },
	}
	out, code, msg := execImportMultiWallet("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(req) + "]")})
	if code != 0 {
		t.Fatalf("importmulti: %s", msg)
	}
	rows := out.([]map[string]interface{})
	if len(rows) != 1 || rows[0]["success"] != true {
		t.Fatalf("result %#v", out)
	}
	if len(w.WatchScripts()) != 1 {
		t.Fatal("expected watch script")
	}
}
