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
	"dogego/consensus"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestImportDescriptorsBareMulti(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "multi(1," + h1 + "," + h2 + ")"
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif1, _ := chain.EncodeWIF(k1.Serialize(), p.PrivKeyWIFVersion, true)
	paths := importDescTestPaths(w, p)
	paths.Utxo = store.NewUtxoCache()
	req, _ := json.Marshal(map[string]interface{}{"desc": desc, "keys": []string{wif1}})
	_, code, msg := execImportDescriptors("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(req) + "]")})
	if code != 0 {
		t.Fatalf("importdescriptors: %s", msg)
	}
	nReq, _, err := consensus.ParseMultisigRedeemScript(w.WatchScripts()[0])
	if err != nil || nReq != 1 {
		t.Fatalf("watch script not bare 1-of-2 multisig: n=%d err=%v", nReq, err)
	}
}

func TestImportDescriptorsBareMultiDenied(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	desc := "multi(1," + h1 + ")"
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := importDescTestPaths(w, p)
	paths.Standard = func() consensus.StandardPolicy {
		return consensus.StandardPolicy{AllowBareMultisig: false}
	}
	req, _ := json.Marshal(map[string]interface{}{"desc": desc})
	row, ok := importDescriptorOne("test", paths, p, req)
	if ok || row["success"] == true {
		t.Fatalf("expected failure got %#v", row)
	}
	errObj := row["error"].(map[string]interface{})
	if errObj["message"] != "importdescriptors: bare multisig descriptors require permitbaremultisig" {
		t.Fatalf("message=%v", errObj["message"])
	}
}

func TestFundRawTransactionBareMulti(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "multi(1," + h1 + "," + h2 + ")"
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif1, _ := chain.EncodeWIF(k1.Serialize(), p.PrivKeyWIFVersion, true)
	paths := importDescTestPaths(w, p)
	paths.Utxo = store.NewUtxoCache()
	req, _ := json.Marshal(map[string]interface{}{"desc": desc, "keys": []string{wif1}})
	_, code, msg := execImportDescriptors("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(req) + "]")})
	if code != 0 {
		t.Fatalf("importdescriptors: %s", msg)
	}
	msScript := w.WatchScripts()[0]
	_ = paths.Utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2e8, PkScript: msScript}},
	}}}, 0)
	payTo, _ := chain.RandomP2PKHAddress(p)
	outJ, _ := json.Marshal(map[string]interface{}{payTo: 0.5})
	inpJ, _ := json.Marshal([]interface{}{})
	rawHex, code, msg := execCreateRawTransaction("test", []json.RawMessage{inpJ, outJ})
	if code != 0 {
		t.Fatalf("create: %s", msg)
	}
	res, code2, msg2 := execFundRawTransaction("test", paths, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"` + rawHex.(string) + `"`),
	})
	if code2 != 0 {
		t.Fatalf("fund: %d %s", code2, msg2)
	}
	if res.(map[string]interface{})["hex"] == "" {
		t.Fatal("expected funded hex")
	}
}
