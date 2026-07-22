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
	"dogego/mempool"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestSendToAddressImportDescriptorsShCLTV(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "sh(cltv(100)multi(1," + h1 + "," + h2 + "))"
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
	p2sh := w.WatchScripts()[0]
	_ = paths.Utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2e8, PkScript: p2sh}},
	}}}, 0)
	dest, _ := chain.RandomP2PKHAddress(p)
	pool := mempool.New(100)
	txid, code2, msg2 := walletBuildSignBroadcast("test", paths, nil, pool, nil, nil,
		map[string]float64{dest: 0.5}, nil, true, chain.RebootTestnet, "sendtoaddress", nil, nil)
	if code2 != 0 {
		t.Fatalf("send: %d %s", code2, msg2)
	}
	txidStr, _ := txid.(string)
	if !pool.ContainsTxID(txidStr) {
		t.Fatalf("tx %s not in mempool", txidStr)
	}
}
