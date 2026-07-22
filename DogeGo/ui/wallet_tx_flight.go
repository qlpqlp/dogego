// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"dogego/mempool"
	"dogego/rpc"
	"dogego/ui/websecurity"
	"dogego/wire"
)

func registerWalletTxFlightRoutes(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	mux.HandleFunc("/api/wallet/tx-flight", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		txid := strings.TrimSpace(r.URL.Query().Get("txid"))
		if txid == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "txid required"})
			return
		}
		_ = json.NewEncoder(w).Encode(walletTxFlightStatus(cfg, txid))
	})
	mux.HandleFunc("/api/wallet/broadcast", func(w http.ResponseWriter, r *http.Request) {
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
		var body struct {
			Hex string `json:"hex"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
			return
		}
		h := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(body.Hex), "0x"))
		if h == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "hex required"})
			return
		}
		if _, err := hex.DecodeString(h); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid hex"})
			return
		}
		out := walletBroadcastHex(cfg, h)
		if out["error"] != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(out)
	})
}

func walletBroadcastHex(cfg StartConfig, hexStr string) map[string]interface{} {
	if cfg.RPCInvoke == nil {
		return map[string]interface{}{"error": "broadcast not available yet"}
	}
	param, _ := json.Marshal(hexStr)
	res := cfg.RPCInvoke("sendrawtransaction", []json.RawMessage{param})
	if errObj, ok := res["error"].(map[string]interface{}); ok && errObj != nil {
		msg, _ := errObj["message"].(string)
		code, _ := errObj["code"].(float64)
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "already") || strings.Contains(lower, "txn-already") || strings.Contains(lower, "known") {
			txid := txidFromRawHex(hexStr)
			return map[string]interface{}{
				"txid":   txid,
				"status": "mempool",
				"note":   msg,
			}
		}
		return map[string]interface{}{
			"error": msg,
			"code":  int(code),
		}
	}
	txid, _ := res["result"].(string)
	if txid == "" {
		txid = txidFromRawHex(hexStr)
	}
	return map[string]interface{}{
		"txid":   txid,
		"status": "broadcasting",
	}
}

func txidFromRawHex(hexStr string) string {
	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return ""
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(mempool.TxIDDisplayHex(tx.TxHash()))
}

func walletTxFlightStatus(cfg StartConfig, txid string) map[string]interface{} {
	out := map[string]interface{}{
		"txid":          strings.ToLower(txid),
		"status":        "unknown",
		"confirmations": int64(0),
		"in_mempool":    false,
		"source":        "",
	}
	txid = strings.ToLower(strings.TrimSpace(txid))
	if cfg.Pool != nil && cfg.Pool.ContainsTxID(txid) {
		out["in_mempool"] = true
		out["status"] = "mempool"
		out["source"] = "mempool"
	}
	canChain := cfg.TxIndex != nil && cfg.RawBlocks != nil
	canPool := cfg.Pool != nil
	if canChain || canPool {
		if _, rawTx, src, err := rpc.LookupTxExplorer(cfg.TxIndex, cfg.RawBlocks, cfg.Pool, txid); err == nil {
			out["source"] = src
			if src == "mempool" {
				out["in_mempool"] = true
				out["status"] = "mempool"
			} else if src == "chain" {
				out["status"] = "confirmed"
				out["confirmations"] = int64(1)
				out["in_mempool"] = false
			}
			_ = rawTx
		}
	}
	if cfg.RPCInvoke != nil {
		param, _ := json.Marshal(txid)
		res := cfg.RPCInvoke("getrawtransaction", []json.RawMessage{param, json.RawMessage(`true`)})
		if result, ok := res["result"].(map[string]interface{}); ok {
			if conf, ok := result["confirmations"].(float64); ok {
				out["confirmations"] = int64(conf)
				if conf >= 1 {
					out["status"] = "confirmed"
					out["in_mempool"] = false
				} else if conf == 0 {
					out["status"] = "mempool"
					out["in_mempool"] = true
				}
			}
		}
	}
	if cfg.WalletTxs != nil {
		if page, ok := cfg.WalletTxs.ListPage(0, 200, txid, "all"); ok {
			for _, item := range page.Items {
				m, _ := item.(map[string]interface{})
				if m == nil {
					continue
				}
				id, _ := m["txid"].(string)
				if !strings.EqualFold(id, txid) {
					continue
				}
				if conf, ok := m["confirmations"].(float64); ok && int64(conf) > out["confirmations"].(int64) {
					out["confirmations"] = int64(conf)
				}
				if out["status"] == "unknown" && m["category"] != nil {
					out["status"] = "mempool"
				}
			}
		}
	}
	conf := out["confirmations"].(int64)
	if conf >= 1 {
		out["status"] = "confirmed"
	} else if out["in_mempool"] == true {
		out["status"] = "mempool"
	}
	return out
}
