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

type securityRouteOpts struct {
	AllowRemoteUnlock bool
	RemoteAuthActive  bool
	SecureCookies     bool
}

func registerSecurityRoutes(mux *http.ServeMux, gate *websecurity.Gate, opts securityRouteOpts) {
	checkLoopback := func(w http.ResponseWriter, r *http.Request) bool {
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return false
		}
		return true
	}
	checkSessionRoute := func(w http.ResponseWriter, r *http.Request) bool {
		if !isLoopback(r) && !opts.AllowRemoteUnlock {
			http.Error(w, "forbidden", http.StatusForbidden)
			return false
		}
		return true
	}
	mux.HandleFunc("/api/security/status", func(w http.ResponseWriter, r *http.Request) {
		if !checkSessionRoute(w, r) || r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if gate == nil {
			st := websecurity.StatusNoPIN()
			st["remote_auth_active"] = opts.RemoteAuthActive
			_ = json.NewEncoder(w).Encode(st)
			return
		}
		st := gate.Status(r)
		st["remote_auth_active"] = opts.RemoteAuthActive
		_ = json.NewEncoder(w).Encode(st)
	})
	if gate == nil {
		return
	}
	mux.HandleFunc("/api/security/setup", func(w http.ResponseWriter, r *http.Request) {
		if !checkLoopback(w, r) || r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			CurrentPIN string `json:"current_pin"`
			NewPIN     string `json:"new_pin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeSecurityErr(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := gate.SetupPIN(body.CurrentPIN, body.NewPIN); err != nil {
			writeSecurityErr(w, err.Error(), http.StatusBadRequest)
			return
		}
		tok, err := gate.UnlockPIN(body.NewPIN)
		if err != nil {
			writeSecurityErr(w, err.Error(), http.StatusBadRequest)
			return
		}
		websecurity.SetSessionCookie(w, tok, opts.SecureCookies)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "unlocked": true})
	})
	mux.HandleFunc("/api/security/unlock", func(w http.ResponseWriter, r *http.Request) {
		if !checkSessionRoute(w, r) || r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			PIN string `json:"pin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeSecurityErr(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		tok, err := gate.UnlockPIN(body.PIN)
		if err != nil {
			writeSecurityErr(w, err.Error(), http.StatusForbidden)
			return
		}
		websecurity.SetSessionCookie(w, tok, opts.SecureCookies)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "unlocked": true})
	})
	mux.HandleFunc("/api/security/lock", func(w http.ResponseWriter, r *http.Request) {
		if !checkSessionRoute(w, r) || r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		gate.Lock()
		websecurity.ClearSessionCookie(w)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	mux.HandleFunc("/api/security/disable", func(w http.ResponseWriter, r *http.Request) {
		if !checkLoopback(w, r) || r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			PIN string `json:"pin"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeSecurityErr(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := gate.DisablePIN(body.PIN); err != nil {
			writeSecurityErr(w, err.Error(), http.StatusForbidden)
			return
		}
		websecurity.ClearSessionCookie(w)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	// WebAuthn registration unlock path: same PIN session; biometric uses platform authenticator
	// to confirm presence then unlock via PIN the user set (no PIN stored in browser).
	mux.HandleFunc("/api/security/webauthn/register/begin", func(w http.ResponseWriter, r *http.Request) {
		if !checkLoopback(w, r) || r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !gate.RequireUnlocked(w, r) {
			return
		}
		sessionID, pk, err := gate.BeginWebAuthnRegister(r)
		if err != nil {
			writeSecurityErr(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"session_id": sessionID, "publicKey": pk})
	})
	mux.HandleFunc("/api/security/webauthn/register/finish", func(w http.ResponseWriter, r *http.Request) {
		if !checkLoopback(w, r) || r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !gate.RequireUnlocked(w, r) {
			return
		}
		var body struct {
			SessionID  string          `json:"session_id"`
			Credential json.RawMessage `json:"credential"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SessionID == "" || len(body.Credential) == 0 {
			writeSecurityErr(w, "session_id and credential required", http.StatusBadRequest)
			return
		}
		if err := gate.FinishWebAuthnRegister(r, body.SessionID, body.Credential); err != nil {
			writeSecurityErr(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "webauthn_enabled": true})
	})
	mux.HandleFunc("/api/security/webauthn/login/begin", func(w http.ResponseWriter, r *http.Request) {
		if !checkSessionRoute(w, r) || r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sessionID, pk, err := gate.BeginWebAuthnLogin(r)
		if err != nil {
			writeSecurityErr(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"session_id": sessionID, "publicKey": pk})
	})
	mux.HandleFunc("/api/security/webauthn/login/finish", func(w http.ResponseWriter, r *http.Request) {
		if !checkSessionRoute(w, r) || r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			SessionID  string          `json:"session_id"`
			Credential json.RawMessage `json:"credential"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SessionID == "" || len(body.Credential) == 0 {
			writeSecurityErr(w, "session_id and credential required", http.StatusBadRequest)
			return
		}
		tok, err := gate.FinishWebAuthnLogin(r, body.SessionID, body.Credential)
		if err != nil {
			writeSecurityErr(w, err.Error(), http.StatusForbidden)
			return
		}
		websecurity.SetSessionCookie(w, tok, opts.SecureCookies)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "unlocked": true})
	})
	mux.HandleFunc("/api/security/webauthn/clear", func(w http.ResponseWriter, r *http.Request) {
		if !checkLoopback(w, r) || r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !gate.RequireUnlocked(w, r) {
			return
		}
		if err := gate.ClearWebAuthnCredentials(); err != nil {
			writeSecurityErr(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
}

func writeSecurityErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": strings.TrimSpace(msg)})
}
