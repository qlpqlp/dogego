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
)

func TestWalletSendErrorResponseImmatureOnly(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	// tip 300; height 100 → 201 conf < 240 maturity → immature coinbase only.
	addWalletFastUtxo(cfg.UtxoCache(), 1, 0, 10_000_000_000, 100, spk)
	out := walletSendErrorResponse(-6, "Insufficient funds", cfg)
	if out["immature_only"] != true {
		t.Fatalf("expected immature_only: %#v", out)
	}
	hint, _ := out["fee_hint"].(string)
	if hint == "" || strings.Contains(hint, "Raise the fee rate") {
		t.Fatalf("unexpected fee-style hint: %q", hint)
	}
	if !strings.Contains(hint, "240 blocks") {
		t.Fatalf("hint should mention maturity: %q", hint)
	}
}

func TestWalletSendErrorResponseFeeHint(t *testing.T) {
	out := walletSendErrorResponse(-6, "Insufficient funds", StartConfig{})
	if out["fee_hint"] == nil || out["suggested_fee_rate"] == nil {
		t.Fatalf("missing fee guidance: %#v", out)
	}
	if out["estimated_fee_doge"].(float64) <= 0 {
		t.Fatalf("estimated_fee_doge=%v", out["estimated_fee_doge"])
	}
}

func TestWalletSendErrorResponseNonFee(t *testing.T) {
	out := walletSendErrorResponse(-13, "wallet locked", StartConfig{})
	if _, ok := out["fee_hint"]; ok {
		t.Fatalf("unexpected fee_hint: %#v", out)
	}
}

func TestWalletSendHTTPInsufficientFundsFeeHint(t *testing.T) {
	cfg, addr, _ := testWalletFastSetup(t)
	bridge := &WalletSendBridge{}
	bridge.Set(func(string, float64, map[string]interface{}) (string, int, string) {
		return "", -6, "Insufficient funds"
	})
	cfg.WalletSend = bridge
	mux := http.NewServeMux()
	registerWalletSendRoute(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"address":"` + addr + `","amount":1.0}`
	resp, err := http.Post(srv.URL+"/api/wallet/send", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", resp.StatusCode)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["fee_hint"] == nil || out["suggested_fee_rate"] == nil {
		t.Fatalf("missing fee guidance: %#v", out)
	}
	if out["code"].(float64) != -6 {
		t.Fatalf("code=%v want -6", out["code"])
	}
}

func TestWalletSendHTTPResponsePQMeta(t *testing.T) {
	cfg, addr, _ := testWalletFastSetup(t)
	txid, hx := testPQSendTxHex(t)
	bridge := &WalletSendBridge{}
	bridge.SetDetailed(func(string, float64, map[string]interface{}) (WalletSendDetailed, int, string) {
		return WalletSendDetailed{Txid: txid, Hex: hx, Status: "mempool"}, 0, ""
	})
	cfg.WalletSend = bridge
	mux := http.NewServeMux()
	registerWalletSendRoute(mux, cfg, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"address":"` + addr + `","amount":1.0}`
	resp, err := http.Post(srv.URL+"/api/wallet/send", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d want 200", resp.StatusCode)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["tx_kind"] != "sent_pq" {
		t.Fatalf("tx_kind %#v want sent_pq", out["tx_kind"])
	}
	if out["pq_tag"] == nil || out["pq_tag"] == "" {
		t.Fatalf("missing pq_tag: %#v", out)
	}
}
