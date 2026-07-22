// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/store"
	"dogego/wallet"
)

func TestWalletTxRowCacheKey(t *testing.T) {
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(42)
	paths := &DataPaths{Utxo: utxo}
	if got := walletTxRowCacheKey("testnet", paths); got != "testnet:42" {
		t.Fatalf("key=%q", got)
	}
	if got := walletTxRowCacheKey("testnet", nil); got != "testnet:0" {
		t.Fatalf("nil utxo key=%q", got)
	}
}

func TestWalletCollectTransactionsUILightUsesHeightAsTime(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(300)
	var op [36]byte
	op[0] = 9
	utxo.AddUtxoForTest(op, store.UtxoEntry{Value: 1e9, PkScript: spendPK, Height: 42})

	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletSpendScripts: func() [][]byte { return [][]byte{append([]byte(nil), spendPK...)} },
	}
	j := &memJournal{tip: 300}
	rows, code, msg := walletCollectTransactionsUI("testnet", paths, j, nil, nil, 1)
	if code != 0 {
		t.Fatalf("collect: %s", msg)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].time != 42 || rows[0].blockTime != 42 {
		t.Fatalf("light row time=%d blockTime=%d want height 42", rows[0].time, rows[0].blockTime)
	}
	if rows[0].blockHash != "" {
		t.Fatalf("light row should skip blockhash lookup, got %q", rows[0].blockHash)
	}
}

func TestWalletListTransactionsPageFromUtxo(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(300)
	for i := byte(1); i <= 3; i++ {
		var op [36]byte
		op[0] = i
		utxo.AddUtxoForTest(op, store.UtxoEntry{Value: int64(i) * 1e8, PkScript: spendPK, Height: int64(200 + i)})
	}
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletSpendScripts: func() [][]byte { return [][]byte{append([]byte(nil), spendPK...)} },
	}
	j := &memJournal{tip: 300}
	page := WalletListTransactionsPage("testnet", paths, j, nil, nil, nil, 1, 1, "", "all")
	if page.Total != 3 || len(page.Items) != 1 {
		t.Fatalf("page total=%d items=%d", page.Total, len(page.Items))
	}
}

func TestWalletUIRowsCacheHit(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	var op [36]byte
	op[0] = 1
	utxo.AddUtxoForTest(op, store.UtxoEntry{Value: 1e8, PkScript: spendPK, Height: 10})
	paths := &DataPaths{
		Utxo:              utxo,
		WalletAddress:     func() string { return w.Address() },
		WalletSpendScripts: func() [][]byte { return [][]byte{append([]byte(nil), spendPK...)} },
	}
	j := &memJournal{tip: 100}
	first := walletUIRowsCached("testnet", paths, j, nil, nil)
	if len(first) != 1 {
		t.Fatalf("first len=%d", len(first))
	}
	second := walletUIRowsCached("testnet", paths, j, nil, nil)
	if len(second) != 1 || second[0].txid != first[0].txid {
		t.Fatalf("cache miss second=%#v first=%#v", second, first)
	}
	utxo.SetTipHeightForTest(101)
	third := walletUIRowsCached("testnet", paths, j, nil, nil)
	if len(third) != 1 {
		t.Fatalf("after tip bump len=%d", len(third))
	}
}

func TestExecListTransactionsWalletManyUtxosLightPath(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(500)
	for i := 1; i <= 120; i++ {
		var op [36]byte
		op[0] = byte(i)
		utxo.AddUtxoForTest(op, store.UtxoEntry{Value: int64(i) * 1e7, PkScript: spendPK, Height: int64(300 + i)})
	}
	paths := &DataPaths{
		Utxo:               utxo,
		WalletAddress:      func() string { return w.Address() },
		WalletSpendScripts: func() [][]byte { return [][]byte{append([]byte(nil), spendPK...)} },
	}
	j := &memJournal{tip: 500}
	res, code, msg := execListTransactionsWallet("testnet", paths, j, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"*"`), json.RawMessage(`5`), json.RawMessage(`0`), json.RawMessage(`false`),
	})
	if code != 0 {
		t.Fatalf("listtransactions: %s", msg)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 5 {
		t.Fatalf("want 5 rows, got %T len=%d", res, len(arr))
	}
}

func TestExecListTransactionsWalletManyUtxosUsesScanIndex(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	addr := w.Address()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(900)
	for i := 1; i <= 500; i++ {
		var op [36]byte
		binary.BigEndian.PutUint16(op[:2], uint16(i))
		utxo.AddUtxoForTest(op, store.UtxoEntry{Value: int64(i) * 1e7, PkScript: spendPK, Height: int64(400 + i)})
	}
	scanRows := make([]wallet.ScannedTx, 0, 8)
	for i := 1; i <= 8; i++ {
		scanRows = append(scanRows, wallet.ScannedTx{
			TxID: repeatHex(byte('a' + i)), Category: "receive", Address: addr,
			AmountKoinu: int64(i) * 1e8, BlockHeight: int64(800 + i), Vout: uint32(i),
		})
	}
	paths := &DataPaths{
		Utxo:               utxo,
		WalletAddress:      func() string { return addr },
		WalletSpendScripts: func() [][]byte { return [][]byte{append([]byte(nil), spendPK...)} },
		WalletListScannedTx: func() []wallet.ScannedTx {
			return scanRows
		},
	}
	j := &memJournal{tip: 900}
	walletTxRowCache.mu.Lock()
	walletTxRowCache.rows = nil
	walletTxRowCache.key = ""
	walletTxRowCache.mu.Unlock()
	res, code, msg := execListTransactionsWallet("testnet", paths, j, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"*"`), json.RawMessage(`5`), json.RawMessage(`0`), json.RawMessage(`false`),
	})
	if code != 0 {
		t.Fatalf("listtransactions: %s", msg)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 5 {
		t.Fatalf("want 5 scan-index rows, got %T len=%d", res, len(arr))
	}
	first, ok := arr[0].(map[string]interface{})
	if !ok || first["category"] != "receive" {
		t.Fatalf("first row %#v", arr[0])
	}
}

func TestExecListTransactionsSendFeeFromWalletDB(t *testing.T) {
	sendTxid := repeatHex('d')
	const feeKoinu = int64(10_000_000)
	paths := &DataPaths{
		WalletAddress: func() string { return "DAddr" },
		WalletSpendScripts: func() [][]byte {
			return [][]byte{{0x76, 0xa9, 0x14, 0x00, 0x88, 0xac}}
		},
		WalletListScannedTx: func() []wallet.ScannedTx {
			return []wallet.ScannedTx{{
				TxID: sendTxid, Category: "send", Address: "DAddr",
				AmountKoinu: -5_000_000_000, BlockHeight: 100, Vout: 0,
			}}
		},
		WalletSendFeeLookup: func(id string) (int64, bool) {
			if id == sendTxid {
				return feeKoinu, true
			}
			return 0, false
		},
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(200)
	paths.Utxo = utxo
	j := &memJournal{tip: 200}
	walletTxRowCache.mu.Lock()
	walletTxRowCache.rows = nil
	walletTxRowCache.key = ""
	walletTxRowCache.mu.Unlock()
	res, code, msg := execListTransactionsWallet("testnet", paths, j, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"*"`), json.RawMessage(`10`), json.RawMessage(`0`), json.RawMessage(`false`),
	})
	if code != 0 {
		t.Fatalf("listtransactions: %s", msg)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("want 1 send row, got %T len=%d", res, len(arr))
	}
	row, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("row type %T", arr[0])
	}
	if row["category"] != "send" {
		t.Fatalf("category=%v", row["category"])
	}
	fee, ok := row["fee"].(float64)
	if !ok || fee != float64(feeKoinu)/1e8 {
		t.Fatalf("fee=%v want %v", row["fee"], float64(feeKoinu)/1e8)
	}
}

func TestExecGetTransactionWalletHexAndFeeFromWalletDB(t *testing.T) {
	sendTxid := repeatHex('e')
	const feeKoinu = int64(2_500_000)
	wantHex := "0100000001abcdef"
	paths := &DataPaths{
		WalletAddress: func() string { return "DAddr" },
		WalletSpendScripts: func() [][]byte {
			return [][]byte{{0x76, 0xa9, 0x14, 0x00, 0x88, 0xac}}
		},
		WalletListScannedTx: func() []wallet.ScannedTx {
			return []wallet.ScannedTx{{
				TxID: sendTxid, Category: "send", Address: "DAddr",
				AmountKoinu: -1_000_000_000, BlockHeight: 50, Vout: 0,
			}}
		},
		WalletSendFeeLookup: func(id string) (int64, bool) {
			if id == sendTxid {
				return feeKoinu, true
			}
			return 0, false
		},
		WalletTxHexLookup: func(id string) (string, bool) {
			if id == sendTxid {
				return wantHex, true
			}
			return "", false
		},
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	paths.Utxo = utxo
	j := &memJournal{tip: 100}
	walletTxRowCache.mu.Lock()
	walletTxRowCache.rows = nil
	walletTxRowCache.key = ""
	walletTxRowCache.mu.Unlock()
	txidJ, _ := json.Marshal(sendTxid)
	res, code, msg := execGetTransactionWallet("testnet", paths, j, nil, nil, nil, []json.RawMessage{txidJ})
	if code != 0 {
		t.Fatalf("gettransaction: %s", msg)
	}
	entry, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result type %T", res)
	}
	if entry["hex"] != wantHex {
		t.Fatalf("hex=%v want %q", entry["hex"], wantHex)
	}
	fee, ok := entry["fee"].(float64)
	if !ok || fee != float64(feeKoinu)/1e8 {
		t.Fatalf("fee=%v want %v", entry["fee"], float64(feeKoinu)/1e8)
	}
}

func TestExecListSinceBlockSendFeeFromWalletDB(t *testing.T) {
	sendTxid := repeatHex('f')
	const feeKoinu = int64(3_000_000)
	paths := &DataPaths{
		WalletAddress: func() string { return "DAddr" },
		WalletSpendScripts: func() [][]byte {
			return [][]byte{{0x76, 0xa9, 0x14, 0x00, 0x88, 0xac}}
		},
		WalletListScannedTx: func() []wallet.ScannedTx {
			return []wallet.ScannedTx{{
				TxID: sendTxid, Category: "send", Address: "DAddr",
				AmountKoinu: -2_000_000_000, BlockHeight: 80, Vout: 0,
			}}
		},
		WalletSendFeeLookup: func(id string) (int64, bool) {
			if id == sendTxid {
				return feeKoinu, true
			}
			return 0, false
		},
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	paths.Utxo = utxo
	j := &memJournal{tip: 100}
	walletTxRowCache.mu.Lock()
	walletTxRowCache.rows = nil
	walletTxRowCache.key = ""
	walletTxRowCache.mu.Unlock()
	res, code, msg := execListSinceBlockWallet("testnet", paths, j, nil, nil, nil, nil)
	if code != 0 {
		t.Fatalf("listsinceblock: %s", msg)
	}
	out, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result type %T", res)
	}
	txs, ok := out["transactions"].([]interface{})
	if !ok || len(txs) != 1 {
		t.Fatalf("transactions len=%d type=%T", len(txs), out["transactions"])
	}
	row, ok := txs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("row type %T", txs[0])
	}
	fee, ok := row["fee"].(float64)
	if !ok || fee != float64(feeKoinu)/1e8 {
		t.Fatalf("fee=%v want %v", row["fee"], float64(feeKoinu)/1e8)
	}
}
