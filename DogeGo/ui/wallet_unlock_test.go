// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"dogego/chain"
	"dogego/wallet"
)

func TestWalletUnlockAPI(t *testing.T) {
	dir := t.TempDir()
	wpath, err := ensureSetupWallet(dir, "testnet")
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	pass := "api-unlock-test"
	if _, err := w.Encrypt(pass); err != nil {
		t.Fatal(err)
	}
	w2, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if w2.IsUnlocked() {
		t.Fatal("reloaded wallet should be locked")
	}

	mux := http.NewServeMux()
	cfg := StartConfig{Wallet: w2}
	registerWalletUnlockRoutes(mux, cfg, nil, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	locked, msg := walletFileLockedErr(cfg)
	if !locked || msg == "" {
		t.Fatalf("walletFileLockedErr = %v %q", locked, msg)
	}

	body, _ := json.Marshal(map[string]any{"passphrase": "wrong", "timeout": 60})
	resp, err := http.Post(srv.URL+"/api/wallet/unlock", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong pass status %d", resp.StatusCode)
	}
	var bad map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&bad)
	if bad["wallet_locked"] != true {
		t.Fatalf("expected wallet_locked true got %#v", bad)
	}

	body, _ = json.Marshal(map[string]any{"passphrase": pass, "timeout": 120})
	resp2, err := http.Post(srv.URL+"/api/wallet/unlock", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("unlock status %d: %s", resp2.StatusCode, b)
	}
	var ok map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&ok); err != nil {
		t.Fatal(err)
	}
	if ok["unlocked"] != true || ok["private_keys_enabled"] != true {
		t.Fatalf("unlock response %#v", ok)
	}
	if !w2.IsUnlocked() {
		t.Fatal("wallet should be unlocked in memory")
	}

	resp3, err := http.Post(srv.URL+"/api/wallet/lock", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("lock status %d", resp3.StatusCode)
	}
	if w2.IsUnlocked() {
		t.Fatal("wallet should be locked after lock API")
	}
}
