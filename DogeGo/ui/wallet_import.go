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

func registerWalletImportRoutes(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	mux.HandleFunc("/api/wallet/import", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if cfg.Wallet == nil || cfg.RPCInvoke == nil {
			http.Error(w, "wallet disabled", http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Type       string `json:"type"`
			Mnemonic   string `json:"mnemonic"`
			Passphrase string `json:"passphrase"`
			BIP38      string `json:"bip38"`
			Path       string `json:"path"`
			ViaCoreRPC *bool  `json:"via_core_rpc"`
			Rescan     *bool  `json:"rescan"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		rescan := true
		if body.Rescan != nil {
			rescan = *body.Rescan
		}
		rescanJSON, _ := json.Marshal(rescan)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch strings.ToLower(strings.TrimSpace(body.Type)) {
		case "mnemonic", "seed":
			mnJ, _ := json.Marshal(strings.TrimSpace(body.Mnemonic))
			passJ, _ := json.Marshal(body.Passphrase)
			out := cfg.RPCInvoke("dogego_importmnemonic", []json.RawMessage{mnJ, passJ, rescanJSON})
			writeRPCBridge(w, out)
		case "bip38":
			keyJ, _ := json.Marshal(strings.TrimSpace(body.BIP38))
			passJ, _ := json.Marshal(body.Passphrase)
			out := cfg.RPCInvoke("dogego_importbip38", []json.RawMessage{keyJ, passJ, rescanJSON})
			writeRPCBridge(w, out)
		case "walletdat", "wallet.dat", "corewallet":
			path := strings.TrimSpace(body.Path)
			if path == "" {
				http.Error(w, "path required for walletdat import", http.StatusBadRequest)
				return
			}
			pathJ, _ := json.Marshal(path)
			opts := map[string]interface{}{}
			if body.ViaCoreRPC != nil && *body.ViaCoreRPC {
				opts["via_core_rpc"] = true
			}
			if pass := strings.TrimSpace(body.Passphrase); pass != "" {
				opts["passphrase"] = pass
			}
			var rpcParams []json.RawMessage
			if len(opts) > 0 {
				optsJ, _ := json.Marshal(opts)
				rpcParams = []json.RawMessage{pathJ, optsJ}
			} else {
				rpcParams = []json.RawMessage{pathJ}
			}
			out := cfg.RPCInvoke("dogego_importwalletdat", rpcParams)
			writeRPCBridge(w, out)
		default:
			http.Error(w, "type must be mnemonic, bip38, or walletdat", http.StatusBadRequest)
		}
	})

	mux.HandleFunc("/api/wallet/probe-walletdat", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if cfg.RPCInvoke == nil {
			http.Error(w, "wallet disabled", http.StatusBadRequest)
			return
		}
		path := strings.TrimSpace(r.URL.Query().Get("path"))
		if r.Method == http.MethodPost {
			var body struct {
				Path string `json:"path"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil && strings.TrimSpace(body.Path) != "" {
				path = strings.TrimSpace(body.Path)
			}
		} else if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if path == "" {
			http.Error(w, "path query or JSON body required", http.StatusBadRequest)
			return
		}
		pathJ, _ := json.Marshal(path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		out := cfg.RPCInvoke("dogego_probewalletdat", []json.RawMessage{pathJ})
		writeRPCBridge(w, out)
	})

	mux.HandleFunc("/api/wallet/addresses", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !requireWalletRead(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.RPCInvoke == nil {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		out := cfg.RPCInvoke("dogego_listwalletaddresses", nil)
		if errObj, ok := out["error"].(map[string]interface{}); ok && errObj != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errObj)
			return
		}
		if res, ok := out["result"]; ok {
			_ = json.NewEncoder(w).Encode(res)
			return
		}
		_ = json.NewEncoder(w).Encode([]any{})
	})

	mux.HandleFunc("/api/wallet/labels", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !requireWalletRead(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.RPCInvoke == nil {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		out := cfg.RPCInvoke("listlabels", nil)
		if errObj, ok := out["error"].(map[string]interface{}); ok && errObj != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errObj)
			return
		}
		if res, ok := out["result"]; ok {
			_ = json.NewEncoder(w).Encode(res)
			return
		}
		_ = json.NewEncoder(w).Encode([]any{})
	})

	mux.HandleFunc("/api/wallet/address/new", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if cfg.Wallet == nil {
			http.Error(w, "wallet disabled", http.StatusBadRequest)
			return
		}
		if cfg.RPCInvoke == nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    -1,
					"message": "Wallet is still starting. Wait a few seconds and try again.",
				},
			})
			return
		}
		var body struct {
			Label string `json:"label"`
		}
		if r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
		}
		labelJ, _ := json.Marshal(strings.TrimSpace(body.Label))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		out := cfg.RPCInvoke("getnewaddress", []json.RawMessage{labelJ})
		writeRPCBridge(w, out)
	})

	mux.HandleFunc("/api/wallet/address/label", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if cfg.Wallet == nil || cfg.RPCInvoke == nil {
			http.Error(w, "wallet disabled", http.StatusBadRequest)
			return
		}
		var body struct {
			Address string `json:"address"`
			Label   string `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		addr := strings.TrimSpace(body.Address)
		if addr == "" {
			http.Error(w, "address required", http.StatusBadRequest)
			return
		}
		addrJ, _ := json.Marshal(addr)
		labelJ, _ := json.Marshal(body.Label)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		out := cfg.RPCInvoke("setlabel", []json.RawMessage{addrJ, labelJ})
		writeRPCBridge(w, out)
	})
}

func walletRPCWarmupMessage(msg string) string {
	msg = strings.ToLower(strings.TrimSpace(msg))
	if msg == "" {
		return ""
	}
	if strings.Contains(msg, "rpc not ready") ||
		strings.Contains(msg, "wallet is not implemented") ||
		strings.Contains(msg, "wallet is still starting") {
		return "Wallet is still starting. Wait a few seconds and try again."
	}
	return ""
}

func writeRPCBridge(w http.ResponseWriter, out map[string]interface{}) {
	if errObj, ok := out["error"].(map[string]interface{}); ok && errObj != nil {
		msg, _ := errObj["message"].(string)
		if warm := walletRPCWarmupMessage(msg); warm != "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": -1, "message": warm},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": errObj})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": out["result"]})
}
