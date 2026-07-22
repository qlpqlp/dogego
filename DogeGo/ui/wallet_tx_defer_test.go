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
)

func TestWalletTxHistoryDeferReasonScanBuilding(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.RPCInvoke = func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{"initialblockdownload": false}}
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"scanning":                                    map[string]interface{}{"duration": 0},
				"dogego_wallet_listtransactions_scan_pending": true,
				"spendable_utxo_count":                        128,
			}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	if got := walletTxHistoryDeferReason(cfg); got != "scan_building" {
		t.Fatalf("defer=%q want scan_building", got)
	}
}

func TestWalletTxHistoryDeferReasonConnectLag(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.RPCInvoke = func(method string, _ []json.RawMessage) map[string]interface{} {
		if method == "getblockchaininfo" {
			return map[string]interface{}{"result": map[string]interface{}{
				"initialblockdownload": false,
				"dogego_connect_lag":   float64(128),
			}}
		}
		return map[string]interface{}{"result": map[string]interface{}{}}
	}
	if got := walletTxHistoryDeferReason(cfg); got != "connect_lag" {
		t.Fatalf("defer=%q want connect_lag", got)
	}
}

func TestWalletTxHistoryDeferReasonNoneWhenFewUtxos(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.RPCInvoke = func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{"initialblockdownload": false}}
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"scanning":                                    map[string]interface{}{"duration": 0},
				"dogego_wallet_listtransactions_scan_pending": true,
				"spendable_utxo_count":                        8,
			}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	if got := walletTxHistoryDeferReason(cfg); got != "" {
		t.Fatalf("defer=%q want empty", got)
	}
}

func TestWalletTxsCSVDeferredScanBuilding(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.RPCInvoke = func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{"initialblockdownload": false}}
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"scanning":                                 map[string]interface{}{"duration": 0},
				"dogego_wallet_listtransactions_utxo_walk": true,
				"spendable_utxo_count":                     200,
			}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	mux := http.NewServeMux()
	registerWalletTxsRoutes(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/wallet/txs.csv?kind=all")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", resp.StatusCode)
	}
	if resp.Header.Get("X-DogeGo-Wallet-Defer-Reason") != "scan_building" {
		t.Fatalf("defer header=%q", resp.Header.Get("X-DogeGo-Wallet-Defer-Reason"))
	}
}

func TestWalletTxsHTTPDeferredScanBuilding(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.RPCInvoke = func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{"initialblockdownload": false}}
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"scanning":                                    map[string]interface{}{"duration": 0},
				"dogego_wallet_listtransactions_utxo_walk":    true,
				"dogego_wallet_listtransactions_scan_pending": true,
				"spendable_utxo_count":                        200,
			}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	mux := http.NewServeMux()
	registerWalletTxsRoutes(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/wallet/txs?limit=10")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["deferred"] != true || out["defer_reason"] != "scan_building" {
		t.Fatalf("out=%v", out)
	}
}

func TestWalletAPIEnvelopeHistoryDeferred(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.RPCInvoke = func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{"initialblockdownload": false}}
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"scanning":                                 map[string]interface{}{"duration": 0},
				"dogego_wallet_listtransactions_utxo_walk": true,
				"spendable_utxo_count":                     200,
			}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	out := walletAPIEnvelope(cfg)
	if out["wallet_history_deferred"] != true {
		t.Fatalf("wallet_history_deferred=%v", out["wallet_history_deferred"])
	}
	if out["wallet_history_defer_reason"] != "scan_building" {
		t.Fatalf("wallet_history_defer_reason=%v", out["wallet_history_defer_reason"])
	}
}
