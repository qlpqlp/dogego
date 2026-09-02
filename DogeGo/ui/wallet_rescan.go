// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"sync"

	"dogego/ui/websecurity"
	"dogego/wallet"
)

var walletRescanHTTP struct {
	mu  sync.Mutex
	busy bool
}

func attachWalletRescanStatus(out map[string]any, cfg StartConfig) {
	if out == nil || cfg.ActiveWallet() == nil {
		return
	}
	maxH := cfg.ActiveWallet().MaxScannedBlockHeight()
	out["wallet_index_height"] = maxH
	tip := int64(-1)
	if utxo := utxoCacheLive(cfg); utxo != nil {
		tip = utxo.TipHeight()
	}
	if tip >= 0 {
		out["chain_active_height"] = tip
		if maxH < tip {
			out["needs_rescan"] = true
			if maxH < 0 {
				out["rescan_from_height"] = int64(0)
			} else {
				out["rescan_from_height"] = maxH + 1
			}
		}
		if maxH >= 0 {
			out["wallet_scan_index_ok"] = maxH >= tip
		}
	}
	if walletHasReceiveHistory(cfg.ActiveWallet()) {
		out["wallet_history_fast_path"] = true
	} else {
		out["wallet_listtransactions_utxo_walk"] = true
	}
	if cfg.RPCInvoke != nil {
		res := cfg.RPCInvoke("getwalletinfo", nil)
		if info, ok := res["result"].(map[string]interface{}); ok {
			if _, ok := info["scanning"]; ok {
				out["scanning"] = true
			}
			if pending, ok := info["dogego_wallet_listtransactions_scan_pending"].(bool); ok && pending {
				out["wallet_listtransactions_scan_pending"] = true
			}
		}
	}
	walletRescanHTTP.mu.Lock()
	if walletRescanHTTP.busy {
		out["scanning"] = true
	}
	walletRescanHTTP.mu.Unlock()
	if out["scanning"] == true && out["wallet_listtransactions_utxo_walk"] == true {
		out["wallet_listtransactions_scan_pending"] = true
	}
}

func walletHasReceiveHistory(w *wallet.Disk) bool {
	if w == nil {
		return false
	}
	for _, r := range w.ListScannedTx() {
		if r.Category == "receive" {
			return true
		}
	}
	return false
}

func registerWalletRescanRoute(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	mux.HandleFunc("/api/wallet/rescan", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.ActiveWallet() == nil || cfg.RPCInvoke == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "wallet or RPC not ready"})
			return
		}
		switch r.Method {
		case http.MethodPost:
			handleWalletRescanPOST(w, r, cfg)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func handleWalletRescanPOST(w http.ResponseWriter, r *http.Request, cfg StartConfig) {
	if walletRescanBusy(cfg) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "wallet rescan already in progress"})
		return
	}
	var body struct {
		Full        *bool  `json:"full"`
		StartHeight *int64 `json:"start_height"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	start := walletRescanStartHeight(cfg, body.Full, body.StartHeight)
	params := []json.RawMessage{json.RawMessage("null")}
	if start > 0 {
		b, _ := json.Marshal(start)
		params = []json.RawMessage{b}
	}
	setWalletRescanHTTPBusy(true)
	go func() {
		defer setWalletRescanHTTPBusy(false)
		_ = cfg.RPCInvoke("rescan", params)
	}()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":           true,
		"started":      true,
		"start_height": start,
	})
}

func walletRescanStartHeight(cfg StartConfig, full *bool, explicit *int64) int64 {
	if explicit != nil && *explicit >= 0 {
		return *explicit
	}
	if full != nil && *full {
		return 0
	}
	if cfg.ActiveWallet() != nil {
		maxH := cfg.ActiveWallet().MaxScannedBlockHeight()
		if maxH >= 0 {
			return maxH + 1
		}
	}
	return 0
}

func walletRescanBusy(cfg StartConfig) bool {
	if cfg.RPCInvoke != nil {
		res := cfg.RPCInvoke("getwalletinfo", nil)
		if info, ok := res["result"].(map[string]interface{}); ok {
			if _, ok := info["scanning"]; ok {
				return true
			}
		}
	}
	walletRescanHTTP.mu.Lock()
	defer walletRescanHTTP.mu.Unlock()
	return walletRescanHTTP.busy
}

func setWalletRescanHTTPBusy(on bool) {
	walletRescanHTTP.mu.Lock()
	walletRescanHTTP.busy = on
	walletRescanHTTP.mu.Unlock()
}
