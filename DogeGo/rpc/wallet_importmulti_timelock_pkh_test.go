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

func TestImportMultiDescShCLTVPKH(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k, _ := secp256k1.NewPrivateKey()
	wif, _ := chain.EncodeWIF(k.Serialize(), p.PrivKeyWIFVersion, true)
	addr, err := addressFromWIF("test", wif)
	if err != nil {
		t.Fatal(err)
	}
	desc := "sh(cltv(100)pkh(" + addr + "))"
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := json.Marshal(map[string]interface{}{"desc": desc, "keys": []string{wif}})
	paths := importDescTestPaths(w, p)
	batch, _ := json.Marshal([]json.RawMessage{req})
	out, code, msg := execImportMultiWallet("test", paths, nil, nil, []json.RawMessage{batch})
	if code != 0 {
		t.Fatalf("importmulti: %s", msg)
	}
	rows := out.([]map[string]interface{})
	if len(rows) != 1 || rows[0]["success"] != true {
		t.Fatalf("result %#v", out)
	}
	redeem := w.WatchRedeemScript(w.WatchScripts()[0])
	lock, ok := consensus.CLTVLockTimeFromRedeem(redeem)
	if !ok || lock != 100 {
		t.Fatalf("lock=%d ok=%v", lock, ok)
	}
	info, code2, msg2 := execGetAddressInfo("test", paths, mustWalletJSONParam(t, chain.ScriptPubKeyAddress(w.WatchScripts()[0], p.PubkeyHashAddrID, p.ScriptHashAddrID)))
	if code2 != 0 {
		t.Fatalf("getaddressinfo: %s", msg2)
	}
	m, _ := info.(map[string]interface{})
	if d, _ := m["desc"].(string); d != desc {
		t.Fatalf("desc %q want %q", d, desc)
	}
}

func TestImportMultiDescShCSVPKH(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k, _ := secp256k1.NewPrivateKey()
	wif, _ := chain.EncodeWIF(k.Serialize(), p.PrivKeyWIFVersion, true)
	addr, err := addressFromWIF("test", wif)
	if err != nil {
		t.Fatal(err)
	}
	desc := "sh(csv(16)pkh(" + addr + "))"
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := json.Marshal(map[string]interface{}{"desc": desc, "keys": []string{wif}})
	paths := importDescTestPaths(w, p)
	batch, _ := json.Marshal([]json.RawMessage{req})
	out, code, msg := execImportMultiWallet("test", paths, nil, nil, []json.RawMessage{batch})
	if code != 0 {
		t.Fatalf("importmulti: %s", msg)
	}
	rows := out.([]map[string]interface{})
	if len(rows) != 1 || rows[0]["success"] != true {
		t.Fatalf("result %#v", out)
	}
	redeem := w.WatchRedeemScript(w.WatchScripts()[0])
	op, ok := consensus.CSVOperandFromRedeem(redeem)
	if !ok || op != 16 {
		t.Fatalf("csv op=%d ok=%v", op, ok)
	}
}
