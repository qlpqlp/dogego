// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/wallet"
)

func TestEnsureSetupWallet(t *testing.T) {
	dir := t.TempDir()
	wpath, err := ensureSetupWallet(dir, "testnet")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wpath); err != nil {
		t.Fatal(err)
	}
	wpath2, err := ensureSetupWallet(dir, "testnet")
	if err != nil {
		t.Fatal(err)
	}
	if wpath2 != wpath {
		t.Fatalf("expected same path %q got %q", wpath, wpath2)
	}
	ok, err := setupWalletExists(dir, "testnet")
	if err != nil || !ok {
		t.Fatalf("setupWalletExists = %v err %v", ok, err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if w.PayTxFee() != wallet.DefaultPayTxFeeDOGE {
		t.Fatalf("wizard wallet paytxfee %v want %v", w.PayTxFee(), wallet.DefaultPayTxFeeDOGE)
	}
}

func TestSetupWalletBackupHandler(t *testing.T) {
	dir := t.TempDir()
	mux := http.NewServeMux()
	registerSetupWalletBackup(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"datadir":"` + strings.ReplaceAll(dir, `\`, `\\`) + `","network":"testnet","nowallet":false}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/setup/wallet-backup", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil || len(data) < 10 {
		t.Fatalf("short body len=%d err=%v", len(data), err)
	}
}
