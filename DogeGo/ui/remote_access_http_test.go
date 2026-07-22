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

	"dogego/config"
	"dogego/ui/websecurity"
)

func TestLanPeerHintHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/lan-peer-hint", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(BuildLanPeerHint("testnet"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/lan-peer-hint")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var hint LanPeerHint
	if err := json.NewDecoder(resp.Body).Decode(&hint); err != nil {
		t.Fatal(err)
	}
	if hint.P2PPort != 44556 {
		t.Fatalf("port %d", hint.P2PPort)
	}
	if hint.Note == "" {
		t.Fatal("expected note")
	}
}

func TestRequireDashboardReadLoopbackBypassesRemoteAuth(t *testing.T) {
	dir := t.TempDir()
	gate, err := websecurity.NewGate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := gate.SetupPIN("", "123456"); err != nil {
		t.Fatal(err)
	}
	cfg := config.File{WebUIRemoteAuth: true}
	req := httptest.NewRequest(http.MethodGet, "/api/summary", nil)
	req.RemoteAddr = "127.0.0.1:4321"
	rec := httptest.NewRecorder()
	if !requireDashboardRead(rec, req, gate, cfg, "0.0.0.0:2013") {
		t.Fatal("loopback should pass without unlock")
	}
}

func TestRequireDashboardReadRemoteBlocksWithoutSession(t *testing.T) {
	dir := t.TempDir()
	gate, err := websecurity.NewGate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := gate.SetupPIN("", "654321"); err != nil {
		t.Fatal(err)
	}
	cfg := config.File{WebUIRemoteAuth: true}
	req := httptest.NewRequest(http.MethodGet, "/api/summary", nil)
	req.RemoteAddr = "192.168.1.50:4321"
	rec := httptest.NewRecorder()
	if requireDashboardRead(rec, req, gate, cfg, "0.0.0.0:2013") {
		t.Fatal("remote should require unlock")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["pin_required"] != true {
		t.Fatalf("body=%v", body)
	}
}

func TestRequireDashboardReadRemoteAllowsUnlockedSession(t *testing.T) {
	dir := t.TempDir()
	gate, err := websecurity.NewGate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := gate.SetupPIN("", "112233"); err != nil {
		t.Fatal(err)
	}
	tok, err := gate.UnlockPIN("112233")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.File{WebUIRemoteAuth: true}
	req := httptest.NewRequest(http.MethodGet, "/api/summary", nil)
	req.RemoteAddr = "192.168.1.50:4321"
	req.AddCookie(&http.Cookie{Name: "dogego_ui_sess", Value: tok})
	rec := httptest.NewRecorder()
	if !requireDashboardRead(rec, req, gate, cfg, "0.0.0.0:2013") {
		t.Fatal("remote with valid session should pass")
	}
}

func TestRequireDashboardReadRemoteOpenWhenAuthDisabled(t *testing.T) {
	cfg := config.File{WebUIRemoteAuth: false}
	req := httptest.NewRequest(http.MethodGet, "/api/analytics/summary", nil)
	req.RemoteAddr = "192.168.1.50:4321"
	rec := httptest.NewRecorder()
	if !requireDashboardRead(rec, req, nil, cfg, "0.0.0.0:2013") {
		t.Fatal("remote reads open when webui_remote_auth is false")
	}
}
