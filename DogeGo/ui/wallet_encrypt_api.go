// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"strings"

	"dogego/ui/websecurity"
)

func registerWalletEncryptRoutes(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	mux.HandleFunc("/api/wallet/encrypt", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.Wallet == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "wallet disabled"})
			return
		}
		if cfg.Wallet.IsEncrypted() {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "wallet already encrypted"})
			return
		}
		var body struct {
			Passphrase string `json:"passphrase"`
			Confirm    string `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid JSON"})
			return
		}
		pass := strings.TrimSpace(body.Passphrase)
		confirm := strings.TrimSpace(body.Confirm)
		if pass == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "passphrase required"})
			return
		}
		if pass != confirm {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "passphrases do not match"})
			return
		}
		msg, err := cfg.Wallet.Encrypt(pass)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		out := walletUnlockJSON(cfg.Wallet)
		out["ok"] = true
		out["message"] = msg
		_ = json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("/api/wallet/passphrase-change", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.Wallet == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "wallet disabled"})
			return
		}
		if !cfg.Wallet.IsEncrypted() {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "wallet is not encrypted"})
			return
		}
		var body struct {
			OldPassphrase string `json:"old_passphrase"`
			NewPassphrase string `json:"new_passphrase"`
			Confirm       string `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid JSON"})
			return
		}
		oldPass := strings.TrimSpace(body.OldPassphrase)
		newPass := strings.TrimSpace(body.NewPassphrase)
		confirm := strings.TrimSpace(body.Confirm)
		if oldPass == "" || newPass == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "old and new passphrases required"})
			return
		}
		if newPass != confirm {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "new passphrases do not match"})
			return
		}
		if err := cfg.Wallet.ChangePassphrase(oldPass, newPass); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "wallet_locked": strings.Contains(err.Error(), "locked")})
			return
		}
		out := walletUnlockJSON(cfg.Wallet)
		out["ok"] = true
		out["message"] = "passphrase changed"
		_ = json.NewEncoder(w).Encode(out)
	})
}
