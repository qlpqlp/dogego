// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dogego/ui/websecurity"
	"dogego/wallet"
)

func registerWalletBackupRoutes(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	mux.HandleFunc("/api/wallet/backup/download", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if cfg.ActiveWallet() == nil {
			http.Error(w, "wallet disabled", http.StatusBadRequest)
			return
		}
		path := cfg.ActiveWallet().Path()
		if path == "" {
			http.Error(w, "wallet path unknown", http.StatusInternalServerError)
			return
		}
		f, err := os.Open(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		name := "dogego-wallet-" + strings.TrimSpace(cfg.Network) + "-" + time.Now().UTC().Format("20060102") + ".json"
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		_, _ = io.Copy(w, f)
	})

	mux.HandleFunc("/api/wallet/rotate", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.ActiveWallet() == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "wallet disabled"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			st := cfg.ActiveWallet().RotationState()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rotation": st,
			})
			return
		case http.MethodPost:
			// continue below
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Action      string `json:"action"`
			ArchivePath string `json:"archive_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
			return
		}
		action := strings.TrimSpace(strings.ToLower(body.Action))
		switch action {
		case "prepare":
			addr, err := cfg.ActiveWallet().BeginKeyRotation()
			if err != nil {
				writeWalletRotateErr(w, err)
				return
			}
			bal := walletBalanceDOGE(cfg)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":          true,
				"new_address": addr,
				"balance_doge": bal,
				"note":        "Send your full spendable balance to the new address, then run sweep and finalize.",
			})
		case "cancel":
			cfg.ActiveWallet().CancelKeyRotation()
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "sweep":
			dest, ok := cfg.ActiveWallet().PendingRotationAddress()
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "run prepare first"})
				return
			}
			if cfg.RPCInvoke == nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "RPC not ready (wait for sync)"})
				return
			}
			bal := walletBalanceDOGE(cfg)
			if bal <= 0 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "no spendable balance to sweep"})
				return
			}
			amtJSON, _ := json.Marshal(bal)
			params := []json.RawMessage{
				json.RawMessage(`"` + dest + `"`),
				json.RawMessage(amtJSON),
				json.RawMessage(`""`),
				json.RawMessage(`""`),
				json.RawMessage(`true`),
			}
			res := cfg.RPCInvoke("sendtoaddress", params)
			if errMsg, code := rpcResultErr(res); code != 0 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": errMsg, "code": fmt.Sprintf("%d", code)})
				return
			}
			txid, _ := res["result"].(string)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":          true,
				"txid":        txid,
				"amount_doge": bal,
				"destination": dest,
				"note":        "Wait for confirmation, then verify and finalize rotation.",
			})
		case "verify":
			bal := walletBalanceDOGE(cfg)
			spendable := walletSpendableCount(cfg)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":              true,
				"balance_doge":    bal,
				"spendable_utxos": spendable,
				"ready_to_finalize": bal < 0.00001 && spendable == 0,
			})
		case "finalize":
			archive, err := cfg.ActiveWallet().FinalizeKeyRotation()
			if err != nil {
				writeWalletRotateErr(w, err)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":           true,
				"archive_path": archive,
				"new_address":  cfg.ActiveWallet().DefaultAddress(),
				"note":         "Old wallet archived. Delete the archive only after you confirm funds on the new address.",
			})
		case "remove_archive":
			path := strings.TrimSpace(body.ArchivePath)
			if path == "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "archive_path required"})
				return
			}
			if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(cfg.ChainDataDir)) {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "archive must be under chain datadir"})
				return
			}
			if err := wallet.RemoveRotationArchive(path); err != nil {
				writeWalletRotateErr(w, err)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unknown action"})
		}
	})
}

func writeWalletRotateErr(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func walletBalanceDOGE(cfg StartConfig) float64 {
	if cfg.RPCInvoke == nil {
		return 0
	}
	res := cfg.RPCInvoke("getwalletinfo", nil)
	if errMsg, code := rpcResultErr(res); code != 0 {
		_ = errMsg
		// Fallback when getwalletinfo unavailable.
		res = cfg.RPCInvoke("getbalance", nil)
		if _, code := rpcResultErr(res); code != 0 {
			return 0
		}
		switch v := res["result"].(type) {
		case float64:
			return v
		case json.Number:
			f, _ := v.Float64()
			return f
		}
		return 0
	}
	r, ok := res["result"].(map[string]interface{})
	if !ok {
		return 0
	}
	switch v := r["balance"].(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

func walletImmatureBalanceDOGE(cfg StartConfig) float64 {
	if cfg.RPCInvoke == nil {
		return 0
	}
	res := cfg.RPCInvoke("getwalletinfo", nil)
	if _, code := rpcResultErr(res); code != 0 {
		return 0
	}
	r, ok := res["result"].(map[string]interface{})
	if !ok {
		return 0
	}
	switch v := r["immature_balance"].(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

func walletSpendableCount(cfg StartConfig) int {
	if cfg.RPCInvoke == nil {
		return -1
	}
	res := cfg.RPCInvoke("listunspent", []json.RawMessage{json.RawMessage(`[1]`)})
	if _, code := rpcResultErr(res); code != 0 {
		return -1
	}
	if arr, ok := res["result"].([]interface{}); ok {
		return len(arr)
	}
	return -1
}

func rpcResultErr(res map[string]interface{}) (string, int) {
	if res == nil {
		return "no RPC response", -1
	}
	if errObj, ok := res["error"].(map[string]interface{}); ok && errObj != nil {
		msg, _ := errObj["message"].(string)
		code := -1
		if c, ok := errObj["code"].(float64); ok {
			code = int(c)
		}
		if msg == "" {
			msg = "RPC error"
		}
		return msg, code
	}
	return "", 0
}
