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
	"dogego/wallet"
)

const defaultWalletUnlockTimeoutSec = int64(600)

func registerWalletUnlockRoutes(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate, live *LiveFeed) {
	mux.HandleFunc("/api/wallet/unlock", func(w http.ResponseWriter, r *http.Request) {
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
			Passphrase string `json:"passphrase"`
			Timeout    *int64 `json:"timeout"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid JSON"})
			return
		}
		timeout := defaultWalletUnlockTimeoutSec
		if body.Timeout != nil {
			timeout = *body.Timeout
		}
		if err := cfg.Wallet.Unlock(body.Passphrase, timeout); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":         err.Error(),
				"wallet_locked": true,
			})
			return
		}
		if live != nil {
			live.PatchWalletEncryptionStatus(cfg.Wallet)
		}
		_ = json.NewEncoder(w).Encode(walletUnlockJSON(cfg.Wallet))
	})

	mux.HandleFunc("/api/wallet/lock", func(w http.ResponseWriter, r *http.Request) {
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
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "encrypted": false})
			return
		}
		if err := cfg.Wallet.Lock(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		if live != nil {
			live.PatchWalletEncryptionStatus(cfg.Wallet)
		}
		out := walletUnlockJSON(cfg.Wallet)
		out["ok"] = true
		_ = json.NewEncoder(w).Encode(out)
	})
}

func walletUnlockJSON(w *wallet.Disk) map[string]any {
	if w == nil {
		return map[string]any{"ok": true}
	}
	out := map[string]any{
		"ok":        true,
		"encrypted": w.IsEncrypted(),
		"unlocked":  w.IsUnlocked(),
	}
	if until := w.UnlockUntil(); until > 0 {
		out["unlocked_until"] = until
	}
	if w.IsEncrypted() && w.IsUnlocked() {
		out["private_keys_enabled"] = true
	} else if !w.IsEncrypted() {
		out["private_keys_enabled"] = true
	} else {
		out["private_keys_enabled"] = false
	}
	return out
}

func walletFileLockedErr(cfg StartConfig) (locked bool, msg string) {
	if cfg.Wallet == nil || !cfg.Wallet.IsEncrypted() {
		return false, ""
	}
	if cfg.Wallet.IsUnlocked() {
		return false, ""
	}
	msg = "Please enter the wallet passphrase with walletpassphrase first."
	if strings.TrimSpace(msg) == "" {
		msg = wallet.ErrWalletLocked.Error()
	}
	return true, msg
}
