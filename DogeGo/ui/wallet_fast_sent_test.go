// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dogego/wallet"
)

func TestWalletTxsHTTPSentUsesScannedIndexFastPath(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.Wallet.SeedScannedTx([]wallet.ScannedTx{{
		TxID: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		Category: "send", Address: "DSendAddr", AmountKoinu: -5_000_000_000,
		FeeKoinu: 100_000_000, BlockHeight: 100,
	}})
	_ = cfg.Wallet.RememberTxHex("abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234", "deadbeef")

	mux := http.NewServeMux()
	registerWalletTxsRoutes(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs?limit=10&type=sent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out["total"].(float64)) != 1 {
		t.Fatalf("total %#v", out["total"])
	}
	items, ok := out["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("items %#v", out["items"])
	}
	row, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("row %#v", items[0])
	}
	if row["category"] != "send" {
		t.Fatalf("category %#v", row["category"])
	}
	if _, ok := row["fee"]; !ok {
		t.Fatal("missing fee")
	}
	if _, ok := row["hex"]; !ok {
		t.Fatal("missing hex")
	}
}

func TestWalletTxsHTTPMergedAllFastPath(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 50, 0, 2_000_000_000, 200, spk)
	cfg.Wallet.SeedScannedTx([]wallet.ScannedTx{
		{
			TxID: "bbbb1234bbbb1234bbbb1234bbbb1234bbbb1234bbbb1234bbbb1234bbbb1234",
			Category: "receive", Address: cfg.Wallet.Address(), AmountKoinu: 2_000_000_000,
			BlockHeight: 200, Vout: 0,
		},
		{
			TxID: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
			Category: "send", Address: "DSendAddr", AmountKoinu: -1_000_000_000,
			FeeKoinu: 50_000_000, BlockHeight: 300,
		},
	})

	mux := http.NewServeMux()
	registerWalletTxsRoutes(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs?limit=50&type=all")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out["total"].(float64)) != 2 {
		t.Fatalf("total %#v want 2", out["total"])
	}
	items, ok := out["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("items %#v", out["items"])
	}
	first, _ := items[0].(map[string]interface{})
	if first["category"] != "send" {
		t.Fatalf("first category %#v want send (higher block)", first["category"])
	}
}

func TestWalletTxsHTTPPrefersListPageBridge(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.Wallet.SeedScannedTx([]wallet.ScannedTx{{
		TxID: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		Category: "send", Address: "DSendAddr", AmountKoinu: -1_000_000_000,
		FeeKoinu: 50_000_000, BlockHeight: 300,
	}})
	bridge := &WalletTxsBridge{}
	bridge.SetPage(func(offset, limit int, q, kind string) WalletTxPageResult {
		return WalletTxPageResult{
			Total: 1, Offset: 0, Limit: 10,
			Items: []interface{}{map[string]interface{}{"txid": "bridge-wins", "category": "receive"}},
		}
	})
	cfg.WalletTxs = bridge

	mux := http.NewServeMux()
	registerWalletTxsRoutes(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/wallet/txs?limit=10&type=all")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	items, ok := out["items"].([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("items %#v", out["items"])
	}
	row, _ := items[0].(map[string]interface{})
	if row["txid"] != "bridge-wins" {
		t.Fatalf("txid %#v want bridge-wins (ListPage before merged scan path)", row["txid"])
	}
}

func TestWalletTxHistoryUsesScannedSendFastPath(t *testing.T) {
	if !walletTxHistoryUsesScannedSendFastPath("sent") {
		t.Fatal("sent")
	}
	if !walletTxHistoryUsesScannedSendFastPath("quantum") {
		t.Fatal("quantum")
	}
	if walletTxHistoryUsesScannedSendFastPath("received") {
		t.Fatal("received")
	}
}
