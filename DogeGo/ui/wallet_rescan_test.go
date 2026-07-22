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
	"strings"
	"testing"
	"time"

	"dogego/wallet"
)

func TestWalletRescanStartHeight(t *testing.T) {
	cfg := StartConfig{Wallet: &wallet.Disk{}}
	if got := walletRescanStartHeight(cfg, nil, nil); got != 0 {
		t.Fatalf("fresh wallet start=%d want 0", got)
	}
	full := true
	if got := walletRescanStartHeight(cfg, &full, nil); got != 0 {
		t.Fatalf("full start=%d want 0", got)
	}
	explicit := int64(42)
	if got := walletRescanStartHeight(cfg, nil, &explicit); got != 42 {
		t.Fatalf("explicit start=%d want 42", got)
	}
}

func TestWalletAPIEnvelopeRescanFields(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	out := walletAPIEnvelope(cfg)
	if out["chain_active_height"] != int64(300) {
		t.Fatalf("chain_active_height=%v", out["chain_active_height"])
	}
	if out["needs_rescan"] != true {
		t.Fatalf("needs_rescan=%v want true for unindexed wallet", out["needs_rescan"])
	}
	if scanOK, ok := out["wallet_scan_index_ok"].(bool); ok && scanOK {
		t.Fatalf("wallet_scan_index_ok=%v want absent or false when lagging", out["wallet_scan_index_ok"])
	}
}

func TestWalletAPIEnvelopeScanIndexOK(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 9, 0, 1_000_000_000, 300, spk)
	cfg.Wallet.SeedScannedTx([]wallet.ScannedTx{{
		TxID: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		Category: "receive", Address: cfg.Wallet.Address(), AmountKoinu: 1_000_000_000,
		BlockHeight: 300, Vout: 0,
	}})
	out := walletAPIEnvelope(cfg)
	if out["needs_rescan"] == true {
		t.Fatalf("needs_rescan=%v want false when indexed through tip", out["needs_rescan"])
	}
	if out["wallet_scan_index_ok"] != true {
		t.Fatalf("wallet_scan_index_ok=%v want true", out["wallet_scan_index_ok"])
	}
	if out["wallet_history_fast_path"] != true {
		t.Fatalf("wallet_history_fast_path=%v want true", out["wallet_history_fast_path"])
	}
	if _, ok := out["wallet_listtransactions_utxo_walk"]; ok {
		t.Fatalf("wallet_listtransactions_utxo_walk=%v want absent when fast path", out["wallet_listtransactions_utxo_walk"])
	}
}

func TestWalletAPIEnvelopeListtransactionsUtxoWalk(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	out := walletAPIEnvelope(cfg)
	if out["wallet_listtransactions_utxo_walk"] != true {
		t.Fatalf("wallet_listtransactions_utxo_walk=%v want true", out["wallet_listtransactions_utxo_walk"])
	}
	if _, ok := out["wallet_history_fast_path"]; ok {
		t.Fatalf("wallet_history_fast_path=%v want absent", out["wallet_history_fast_path"])
	}
}

func TestWalletAPIEnvelopeScanPending(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.RPCInvoke = func(method string, params []json.RawMessage) map[string]interface{} {
		if method == "getwalletinfo" {
			return map[string]interface{}{"result": map[string]interface{}{
				"scanning":                                    map[string]interface{}{"duration": 0},
				"dogego_wallet_listtransactions_scan_pending": true,
			}}
		}
		return map[string]interface{}{"result": nil}
	}
	out := walletAPIEnvelope(cfg)
	if out["scanning"] != true {
		t.Fatalf("scanning=%v want true", out["scanning"])
	}
	if out["wallet_listtransactions_scan_pending"] != true {
		t.Fatalf("wallet_listtransactions_scan_pending=%v want true", out["wallet_listtransactions_scan_pending"])
	}
}

func TestWalletRescanHTTPPost(t *testing.T) {
	done := make(chan struct{})
	mux := http.NewServeMux()
	registerWalletRescanRoute(mux, StartConfig{
		Wallet: &wallet.Disk{},
		RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
			if method == "getwalletinfo" {
				return map[string]interface{}{"result": map[string]interface{}{}}
			}
			if method == "rescan" {
				close(done)
			}
			return map[string]interface{}{"result": nil}
		},
	}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/wallet/rescan", strings.NewReader(`{"full":true}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("rescan RPC not invoked")
	}
}
