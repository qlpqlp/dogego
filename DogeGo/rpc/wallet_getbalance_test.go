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

	"dogego/chain"
	"dogego/primitives"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestGetBalanceIncludeWatchonly(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	watchPK := append([]byte(nil), spendPK...)
	watchPK[8] ^= 0x01
	_ = w.AddWatchScript(watchPK)
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 2e8, PkScript: spendPK},
			{Value: 5e8, PkScript: watchPK},
		},
	}
	pb := &wire.ParsedBlock{Header: primitives.BlockHeader{Version: 1, Timestamp: 1}, Txs: []*wire.Tx{coin}}
	_ = utxo.ApplyBlock(pb, 0)
	j := &memJournal{tip: 0}
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletSpendScripts: func() [][]byte { return w.SpendScripts() },
		WalletWatchScripts: func() [][]byte { return w.WatchScripts() },
		WalletIsWatchAddress: func(addr string) bool {
			a := chain.ScriptPubKeyAddress(watchPK, p.PubkeyHashAddrID, p.ScriptHashAddrID)
			return addr == a
		},
	}
	bal, code, msg := execGetBalance(paths, j, nil, nil, "test", nil)
	if code != 0 {
		t.Fatalf("getbalance: %s", msg)
	}
	if bal.(float64) != 2.0 {
		t.Fatalf("spendable balance %v want 2", bal)
	}
	incJ, _ := json.Marshal(true)
	bal2, code, msg := execGetBalance(paths, j, nil, nil, "test", []json.RawMessage{
		json.RawMessage(`""`), json.RawMessage(`1`), incJ,
	})
	if code != 0 {
		t.Fatalf("getbalance watch: %s", msg)
	}
	if bal2.(float64) != 7.0 {
		t.Fatalf("with watch %v want 7", bal2)
	}
}

func TestWalletTxRowListEntryWatchFields(t *testing.T) {
	entry := walletTxRowToListEntry("test", &DataPaths{
		WalletIsWatchAddress: func(addr string) bool { return addr == "TWatch" },
	}, nil, nil, nil, nil, "TWatch", walletTxRow{
		blockHeight: 5, blockTime: 1700000000, confirmations: 2,
	})
	if entry["iswatchonly"] != true {
		t.Fatalf("iswatchonly %#v", entry["iswatchonly"])
	}
	if entry["blocktime"] != int64(1700000000) {
		t.Fatalf("blocktime %#v", entry["blocktime"])
	}
}
