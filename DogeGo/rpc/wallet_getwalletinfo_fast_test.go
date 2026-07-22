// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/store"
	"dogego/wallet"
)

func TestExecGetWalletInfoCompactIndexImmatureHeuristic(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(300)
	var op1, op2 [36]byte
	op1[0] = 1
	op2[0] = 2
	utxo.AddUtxoForTest(op1, store.UtxoEntry{Value: 10_000_000_000, PkScript: spendPK, Height: 100})
	utxo.AddUtxoForTest(op2, store.UtxoEntry{Value: 5_000_000_000, PkScript: spendPK, Height: 50})

	ix := &store.TxIndex{EmbedTx: false}
	j := &memJournal{tip: 300}
	paths := &DataPaths{
		Utxo: utxo,
		WalletMaxScannedBlockHeight: func() int64 { return 250 },
		WalletAddress:               func() string { return w.Address() },
		WalletSpendScripts:          func() [][]byte { return w.SpendScripts() },
	}
	res, code, msg := execGetWalletInfo(paths, j, nil, nil, ix, "testnet", nil)
	if code != 0 {
		t.Fatalf("getwalletinfo: %s", msg)
	}
	info, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result %#v", res)
	}
	if info["immature_balance"].(float64) != 100.0 {
		t.Fatalf("immature_balance %#v want 100", info["immature_balance"])
	}
	if info["balance"].(float64) != 50.0 {
		t.Fatalf("balance %#v want 50", info["balance"])
	}
	if info["spendable_utxo_count"].(int) != 1 {
		t.Fatalf("spendable_utxo_count %#v want 1", info["spendable_utxo_count"])
	}
	if info["txcount"].(int) != 2 {
		t.Fatalf("txcount %#v want 2 UTXO rows", info["txcount"])
	}
	if info["wallet_index_height"].(int64) != 250 {
		t.Fatalf("wallet_index_height %#v want 250", info["wallet_index_height"])
	}
	if info["chain_active_height"].(int64) != 300 {
		t.Fatalf("chain_active_height %#v want 300", info["chain_active_height"])
	}
	if needs, _ := info["needs_rescan"].(bool); !needs {
		t.Fatalf("needs_rescan %#v want true", info["needs_rescan"])
	}
	if scanOK, _ := info["dogego_wallet_scan_index_ok"].(bool); scanOK {
		t.Fatalf("dogego_wallet_scan_index_ok %#v want false when lagging", info["dogego_wallet_scan_index_ok"])
	}
	if info["rescan_from_height"].(int64) != 251 {
		t.Fatalf("rescan_from_height %#v want 251", info["rescan_from_height"])
	}
}

func TestExecGetWalletInfoSignerCmdConfigured(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletAddress:      func() string { return w.Address() },
		WalletSpendScripts: func() [][]byte { return w.SpendScripts() },
	}
	res, code, msg := execGetWalletInfo(paths, nil, nil, nil, nil, "testnet", nil)
	if code != 0 {
		t.Fatalf("getwalletinfo: %s", msg)
	}
	info, _ := res.(map[string]interface{})
	if _, ok := info["signer_cmd_configured"]; ok {
		t.Fatal("expected no signer_cmd_configured when unset")
	}
	paths.SignerCommand = []string{"echo", "mock-hwi"}
	res, code, msg = execGetWalletInfo(paths, nil, nil, nil, nil, "testnet", nil)
	if code != 0 {
		t.Fatalf("getwalletinfo with signer: %s", msg)
	}
	info, _ = res.(map[string]interface{})
	if configured, _ := info["signer_cmd_configured"].(bool); !configured {
		t.Fatalf("signer_cmd_configured=%v", info["signer_cmd_configured"])
	}
}

func TestExecGetWalletInfoScanIndexCaughtUp(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(400)
	paths := &DataPaths{
		Utxo:                        utxo,
		WalletMaxScannedBlockHeight: func() int64 { return 400 },
		WalletAddress:               func() string { return w.Address() },
		WalletSpendScripts:          func() [][]byte { return w.SpendScripts() },
		WalletListScannedTx: func() []wallet.ScannedTx {
			return []wallet.ScannedTx{{
				TxID: "aa", Category: "receive", BlockHeight: 400, AmountKoinu: 1e8,
			}}
		},
	}
	res, code, msg := execGetWalletInfo(paths, nil, nil, nil, nil, "testnet", nil)
	if code != 0 {
		t.Fatalf("getwalletinfo: %s", msg)
	}
	info, _ := res.(map[string]interface{})
	if scanOK, _ := info["dogego_wallet_scan_index_ok"].(bool); !scanOK {
		t.Fatalf("dogego_wallet_scan_index_ok=%v want true", info["dogego_wallet_scan_index_ok"])
	}
	if fast, _ := info["dogego_wallet_history_fast_path"].(bool); !fast {
		t.Fatalf("dogego_wallet_history_fast_path=%v want true", info["dogego_wallet_history_fast_path"])
	}
	if _, ok := info["needs_rescan"]; ok {
		t.Fatalf("needs_rescan=%v want absent when caught up", info["needs_rescan"])
	}
}

func TestExecGetWalletInfoHistoryFastPathPartialIndex(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(400)
	paths := &DataPaths{
		Utxo:                        utxo,
		WalletMaxScannedBlockHeight: func() int64 { return 100 },
		WalletAddress:               func() string { return w.Address() },
		WalletSpendScripts:          func() [][]byte { return w.SpendScripts() },
		WalletListScannedTx: func() []wallet.ScannedTx {
			return []wallet.ScannedTx{{
				TxID: "bb", Category: "receive", BlockHeight: 50, AmountKoinu: 2e8,
			}}
		},
	}
	res, code, msg := execGetWalletInfo(paths, nil, nil, nil, nil, "testnet", nil)
	if code != 0 {
		t.Fatalf("getwalletinfo: %s", msg)
	}
	info, _ := res.(map[string]interface{})
	if fast, _ := info["dogego_wallet_history_fast_path"].(bool); !fast {
		t.Fatalf("dogego_wallet_history_fast_path=%v want true", info["dogego_wallet_history_fast_path"])
	}
	if scanOK, _ := info["dogego_wallet_scan_index_ok"].(bool); scanOK {
		t.Fatalf("dogego_wallet_scan_index_ok=%v want false when lagging", info["dogego_wallet_scan_index_ok"])
	}
}

func TestExecGetWalletInfoListtransactionsUtxoWalk(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(50)
	paths := &DataPaths{
		Utxo:                        utxo,
		WalletMaxScannedBlockHeight: func() int64 { return -1 },
		WalletAddress:               func() string { return w.Address() },
		WalletSpendScripts:          func() [][]byte { return w.SpendScripts() },
		WalletListScannedTx:         func() []wallet.ScannedTx { return nil },
	}
	res, code, msg := execGetWalletInfo(paths, nil, nil, nil, nil, "testnet", nil)
	if code != 0 {
		t.Fatalf("getwalletinfo: %s", msg)
	}
	info, _ := res.(map[string]interface{})
	if walk, _ := info["dogego_wallet_listtransactions_utxo_walk"].(bool); !walk {
		t.Fatalf("dogego_wallet_listtransactions_utxo_walk=%v want true", info["dogego_wallet_listtransactions_utxo_walk"])
	}
}

func TestExecGetWalletInfoListtransactionsScanPending(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(50)
	scanning := true
	paths := &DataPaths{
		Utxo:                        utxo,
		WalletMaxScannedBlockHeight: func() int64 { return -1 },
		WalletAddress:               func() string { return w.Address() },
		WalletSpendScripts:          func() [][]byte { return w.SpendScripts() },
		WalletListScannedTx:         func() []wallet.ScannedTx { return nil },
		WalletIsScanning:            func() bool { return scanning },
	}
	res, code, msg := execGetWalletInfo(paths, nil, nil, nil, nil, "testnet", nil)
	if code != 0 {
		t.Fatalf("getwalletinfo: %s", msg)
	}
	info, _ := res.(map[string]interface{})
	if pending, _ := info["dogego_wallet_listtransactions_scan_pending"].(bool); !pending {
		t.Fatalf("dogego_wallet_listtransactions_scan_pending=%v want true", info["dogego_wallet_listtransactions_scan_pending"])
	}
}

func TestExecGetWalletInfoHistoryDeferScanBuilding(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(300)
	for i := 0; i < 80; i++ {
		var op [36]byte
		op[0] = byte(i + 1)
		utxo.AddUtxoForTest(op, store.UtxoEntry{Value: 1_000_000_000, PkScript: spendPK, Height: 100})
	}
	paths := &DataPaths{
		Utxo:                        utxo,
		WalletMaxScannedBlockHeight: func() int64 { return -1 },
		WalletAddress:               func() string { return w.Address() },
		WalletSpendScripts:          func() [][]byte { return w.SpendScripts() },
		WalletListScannedTx:         func() []wallet.ScannedTx { return nil },
		WalletIsScanning:            func() bool { return true },
	}
	res, code, msg := execGetWalletInfo(paths, nil, nil, nil, nil, "testnet", nil)
	if code != 0 {
		t.Fatalf("getwalletinfo: %s", msg)
	}
	info, _ := res.(map[string]interface{})
	if deferred, _ := info["dogego_wallet_history_deferred"].(bool); !deferred {
		t.Fatalf("dogego_wallet_history_deferred=%v", info["dogego_wallet_history_deferred"])
	}
	if info["dogego_wallet_history_defer_reason"] != "scan_building" {
		t.Fatalf("dogego_wallet_history_defer_reason=%v", info["dogego_wallet_history_defer_reason"])
	}
}

func TestExecGetWalletInfoHistoryDeferConnectLag(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(300)
	paths := &DataPaths{
		Utxo:                        utxo,
		WalletMaxScannedBlockHeight: func() int64 { return 300 },
		WalletAddress:               func() string { return w.Address() },
		WalletSpendScripts:          func() [][]byte { return w.SpendScripts() },
		WalletListScannedTx: func() []wallet.ScannedTx {
			return []wallet.ScannedTx{{
				TxID: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
				Category: "receive", Address: w.Address(), AmountKoinu: 1_000_000_000, BlockHeight: 250, Vout: 0,
			}}
		},
		RawSyncProgress: func() map[string]interface{} {
			return map[string]interface{}{"connect_lag": int64(128)}
		},
	}
	res, code, msg := execGetWalletInfo(paths, nil, nil, nil, nil, "testnet", nil)
	if code != 0 {
		t.Fatalf("getwalletinfo: %s", msg)
	}
	info, _ := res.(map[string]interface{})
	if info["dogego_wallet_history_defer_reason"] != "connect_lag" {
		t.Fatalf("dogego_wallet_history_defer_reason=%v", info["dogego_wallet_history_defer_reason"])
	}
}
