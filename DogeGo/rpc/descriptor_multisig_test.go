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

func TestParseShMultiDescriptor(t *testing.T) {
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "sh(multi(2," + h1 + "," + h2 + "))"
	parsed, ok := parseShMultiDescriptor(desc)
	if !ok || parsed.scriptType != "p2sh-multi" || parsed.multiN != 2 || len(parsed.redeem) == 0 {
		t.Fatalf("parse %#v ok=%v", parsed, ok)
	}
	m, code, msg := execCreateMultisig("test", []json.RawMessage{
		json.RawMessage("2"),
		mustJSONKeys(t, h1, h2),
	})
	if code != 0 {
		t.Fatalf("createmultisig: %s", msg)
	}
	if hex.EncodeToString(parsed.redeem) != m["redeemScript"] {
		t.Fatalf("redeem mismatch")
	}
}

func TestImportDescriptorsShMulti(t *testing.T) {
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "sh(multi(2," + h1 + "," + h2 + "))"
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	addr := w.DefaultAddress()
	paths := &DataPaths{
		WalletAddress:               func() string { return addr },
		WalletDefaultAddress:        func() string { return addr },
		WalletImportWatch:           func(script []byte) error { return w.AddWatchScript(script) },
		WalletSetWatchRedeem:        w.SetWatchRedeem,
		WalletWatchScripts:          func() [][]byte { return w.WatchScripts() },
		WalletAddImportedDescriptor: w.AddImportedDescriptor,
	}
	req, _ := json.Marshal(map[string]interface{}{"desc": desc, "timestamp": 0})
	out, code, msg := execImportDescriptors("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(req) + "]")})
	if code != 0 {
		t.Fatalf("importdescriptors: %s", msg)
	}
	rows := out.([]interface{})
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	row := rows[0].(map[string]interface{})
	if row["success"] != true {
		t.Fatalf("%#v", row)
	}
	if len(w.WatchScripts()) != 1 {
		t.Fatal("expected watch script")
	}
}

func mustJSONKeys(t *testing.T, keys ...string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(keys)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
