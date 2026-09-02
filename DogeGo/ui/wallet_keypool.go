// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"

	"dogego/ui/websecurity"
)

func registerWalletKeypoolRoute(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	mux.HandleFunc("/api/wallet/keypool-refill", func(w http.ResponseWriter, r *http.Request) {
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
		if cfg.ActiveWallet() == nil || cfg.RPCInvoke == nil {
			http.Error(w, "wallet disabled", http.StatusBadRequest)
			return
		}
		var body struct {
			NewSize *int `json:"new_size"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		var params []json.RawMessage
		if body.NewSize != nil {
			nJ, _ := json.Marshal(*body.NewSize)
			params = []json.RawMessage{nJ}
		}
		out := cfg.RPCInvoke("keypoolrefill", params)
		if errObj, ok := out["error"].(map[string]interface{}); ok && errObj != nil {
			writeRPCBridge(w, out)
			return
		}
		resp := map[string]any{"ok": true, "result": out["result"]}
		if info := walletInfoAfterKeypoolRefill(cfg); info != nil {
			if v, ok := info["keypoolsize"].(float64); ok && v > 0 {
				resp["keypool_size"] = int(v)
			}
			if v, ok := info["keypoolsize_hd_internal"].(float64); ok && v > 0 {
				resp["change_keypool_size"] = int(v)
			}
			if v, ok := info["pool_core_indices_stored"].(float64); ok && v > 0 {
				resp["pool_core_indices_stored"] = int(v)
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func walletInfoAfterKeypoolRefill(cfg StartConfig) map[string]interface{} {
	if cfg.RPCInvoke == nil {
		return nil
	}
	res := cfg.RPCInvoke("getwalletinfo", nil)
	if _, code := rpcResultErr(res); code != 0 {
		return nil
	}
	info, ok := res["result"].(map[string]interface{})
	if !ok {
		return nil
	}
	return info
}
