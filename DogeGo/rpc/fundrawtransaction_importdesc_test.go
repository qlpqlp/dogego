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
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestFundRawTransactionImportDescriptorsShMultiSolvable(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "sh(multi(1," + h1 + "," + h2 + "))"
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif1, _ := chain.EncodeWIF(k1.Serialize(), p.PrivKeyWIFVersion, true)
	paths := walletTestPaths(w, p)
	paths.Utxo = store.NewUtxoCache()
	paths.WalletImportPrivKey = func(s string) error {
		return w.ImportPrivKey(s, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
	}
	paths.WalletImportWatch = func(script []byte) error { return w.AddWatchScript(script) }
	paths.WalletSetWatchRedeem = w.SetWatchRedeem
	paths.WalletAddImportedDescriptor = w.AddImportedDescriptor
	paths.WalletWatchScripts = func() [][]byte { return w.WatchScripts() }
	paths.WalletSpendScripts = func() [][]byte { return w.SpendScripts() }
	paths.WalletPeekChangeAddress = w.PeekChangeAddress
	paths.WalletCommitChangeAddress = w.CommitChangeAddress
	req, _ := json.Marshal(map[string]interface{}{"desc": desc, "keys": []string{wif1}})
	_, code, msg := execImportDescriptors("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(req) + "]")})
	if code != 0 {
		t.Fatalf("importdescriptors: %s", msg)
	}
	p2sh := w.WatchScripts()[0]
	fund := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2e8, PkScript: p2sh}},
	}
	_ = paths.Utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{fund}}, 0)
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
	m := res.(map[string]interface{})
	if m["hex"] == nil || m["hex"] == "" {
		t.Fatalf("result %#v", m)
	}
}
