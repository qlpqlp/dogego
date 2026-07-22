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

func TestSendToAddressImportDescriptorsShMulti(t *testing.T) {
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
	paths := importDescTestPaths(w, p)
	paths.Utxo = store.NewUtxoCache()
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
	dest, _ := chain.RandomP2PKHAddress(p)
	pool := mempool.New(100)
	txid, code2, msg2 := walletBuildSignBroadcast("test", paths, nil, pool, nil, nil,
		map[string]float64{dest: 0.5}, nil, true, chain.RebootTestnet, "sendtoaddress", nil, nil)
	if code2 != 0 {
		t.Fatalf("send: %d %s", code2, msg2)
	}
	txidStr, _ := txid.(string)
	if txidStr == "" {
		t.Fatal("expected txid")
	}
	if !pool.ContainsTxID(txidStr) {
		t.Fatalf("tx %s not in mempool", txid)
	}
}

func importDescTestPaths(w *wallet.Disk, p chain.Params) *DataPaths {
	paths := walletTestPaths(w, p)
	paths.WalletPath = func() string { return w.Path() }
	paths.WalletIsWatchAddress = func(addr string) bool {
		return w.IsWatchAddress(addr, p.PubkeyHashAddrID, p.ScriptHashAddrID)
	}
	paths.WalletAddress = func() string { return w.DefaultAddress() }
	paths.WalletDefaultAddress = func() string { return w.DefaultAddress() }
	paths.WalletImportPrivKey = func(s string) error {
		return w.ImportPrivKey(s, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
	}
	paths.WalletImportWatch = func(script []byte) error { return w.AddWatchScript(script) }
	paths.WalletSetWatchRedeem = w.SetWatchRedeem
	paths.WalletAddImportedDescriptor = w.AddImportedDescriptor
	paths.WalletListDescriptors = func(string) []WalletDescriptorRow {
		rows := w.ListDescriptors(p.PubkeyHashAddrID, p.ScriptHashAddrID)
		out := make([]WalletDescriptorRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, WalletDescriptorRow{Desc: r.Desc, Timestamp: r.Timestamp, Active: r.Active, Internal: r.Internal})
		}
		return out
	}
	paths.WalletWatchScripts = func() [][]byte { return w.WatchScripts() }
	paths.WalletWatchRedeemScript = w.WatchRedeemScript
	paths.WalletSpendScripts = func() [][]byte { return w.SpendScripts() }
	paths.WalletWIFs = func() []string {
		wifs, _ := w.AllWIFs(p.PrivKeyWIFVersion)
		return wifs
	}
	paths.WalletPeekChangeAddress = w.PeekChangeAddress
	paths.WalletCommitChangeAddress = w.CommitChangeAddress
	return mergePathsDataDir(paths, filepath.Dir(w.Path()))
}
