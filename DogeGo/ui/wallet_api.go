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

func walletAPIEnvelope(cfg StartConfig) map[string]any {
	out := walletStatusJSON(cfg.Wallet, cfg.Network)
	out["send_ready"] = walletRPCReady(cfg)
	out["address_ready"] = walletAddressReady(cfg)
	attachWalletSignerStatus(out, cfg)
	attachWalletRescanStatus(out, cfg)
	if cfg.Wallet != nil {
		if confirmed, immature, utxos, ok := walletBalanceFromUtxoCache(cfg); ok {
			out["balance"] = confirmed
			if immature > 0 {
				out["immature_balance"] = immature
			}
			out["utxo_count"] = utxos
			out["unconfirmed_balance"] = 0.0
			if cfg.Wallet != nil {
				if fee := cfg.Wallet.PayTxFee(); fee >= 0 {
					out["fee_per_kb"] = fee
				}
			}
			attachWalletHistoryDeferStatus(out, cfg)
			return out
		}
	}
	if cfg.Wallet == nil || cfg.RPCInvoke == nil {
		attachWalletHistoryDeferStatus(out, cfg)
		return out
	}
	res := cfg.RPCInvoke("getwalletinfo", nil)
	if errMsg, code := rpcResultErr(res); code == 0 {
		if info, ok := res["result"].(map[string]interface{}); ok {
			copyWalletInfoBalances(out, info)
			if v, ok := info["spendable_utxo_count"]; ok {
				switch n := v.(type) {
				case float64:
					out["utxo_count"] = int(n)
				case json.Number:
					if i, err := n.Int64(); err == nil {
						out["utxo_count"] = int(i)
					}
				case int:
					out["utxo_count"] = n
				}
			}
			if fee := walletInfoPayTxFee(info); fee >= 0 {
				out["fee_per_kb"] = fee
			}
			mergeWalletInfoKeypool(out, info)
			if configured, _ := info["signer_cmd_configured"].(bool); configured {
				out["signer_cmd_configured"] = true
			}
		}
	} else {
		_ = errMsg
		out["balance"] = walletBalanceDOGE(cfg)
		if imm := walletImmatureBalanceDOGE(cfg); imm > 0 {
			out["immature_balance"] = imm
		}
		if ub := walletUnconfirmedBalanceDOGE(cfg); ub != nil {
			out["unconfirmed_balance"] = *ub
		}
		if n := walletSpendableCount(cfg); n >= 0 {
			out["utxo_count"] = n
		}
		if fee := walletPayTxFeeDOGE(cfg); fee >= 0 {
			out["fee_per_kb"] = fee
		}
	}
	attachWalletHistoryDeferStatus(out, cfg)
	return out
}

func attachWalletSignerStatus(out map[string]any, cfg StartConfig) {
	if out == nil {
		return
	}
	if strings.TrimSpace(cfg.EffectiveFile.SignerCmd) != "" {
		out["signer_cmd_configured"] = true
	}
}

func copyWalletInfoBalances(out map[string]any, info map[string]interface{}) {
	if v, ok := info["balance"].(float64); ok {
		out["balance"] = v
	} else if v, ok := info["balance"].(json.Number); ok {
		if f, err := v.Float64(); err == nil {
			out["balance"] = f
		}
	}
	if v, ok := info["immature_balance"].(float64); ok {
		out["immature_balance"] = v
	} else if v, ok := info["immature_balance"].(json.Number); ok {
		if f, err := v.Float64(); err == nil {
			out["immature_balance"] = f
		}
	}
	if v, ok := info["unconfirmed_balance"].(float64); ok {
		out["unconfirmed_balance"] = v
	} else if v, ok := info["unconfirmed_balance"].(json.Number); ok {
		if f, err := v.Float64(); err == nil {
			out["unconfirmed_balance"] = f
		}
	}
}

func walletInfoPayTxFee(info map[string]interface{}) float64 {
	switch v := info["paytxfee"].(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

func mergeWalletInfoKeypool(out map[string]any, info map[string]interface{}) {
	if out == nil || info == nil {
		return
	}
	if _, ok := out["pool_core_indices_stored"]; !ok {
		switch v := info["pool_core_indices_stored"].(type) {
		case float64:
			if int(v) > 0 {
				out["pool_core_indices_stored"] = int(v)
			}
		case int:
			if v > 0 {
				out["pool_core_indices_stored"] = v
			}
		}
	}
	if _, ok := out["hd_keypool_core_index"]; !ok {
		if v, ok := info["hd_keypool_core_index"]; ok && v != nil {
			out["hd_keypool_core_index"] = v
		}
	}
}

func walletPayTxFeeDOGE(cfg StartConfig) float64 {
	res := cfg.RPCInvoke("getwalletinfo", nil)
	if _, code := rpcResultErr(res); code != 0 {
		return -1
	}
	r, ok := res["result"].(map[string]interface{})
	if !ok {
		return -1
	}
	switch v := r["paytxfee"].(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

func walletUnconfirmedBalanceDOGE(cfg StartConfig) *float64 {
	res := cfg.RPCInvoke("getbalances", nil)
	if _, code := rpcResultErr(res); code != 0 {
		return nil
	}
	r, ok := res["result"].(map[string]interface{})
	if !ok {
		return nil
	}
	mine, ok := r["mine"].(map[string]interface{})
	if !ok {
		return nil
	}
	switch v := mine["untrusted_pending"].(type) {
	case float64:
		return &v
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return nil
		}
		return &f
	}
	return nil
}

func walletListUnspent(cfg StartConfig) []interface{} {
	if cfg.RPCInvoke == nil {
		return nil
	}
	res := cfg.RPCInvoke("listunspent", []json.RawMessage{json.RawMessage(`[1]`)})
	if _, code := rpcResultErr(res); code != 0 {
		return nil
	}
	if arr, ok := res["result"].([]interface{}); ok {
		return arr
	}
	return nil
}

func registerWalletTxsRoutes(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	mux.HandleFunc("/api/wallet/txs", func(w http.ResponseWriter, r *http.Request) {
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
		if cfg.Wallet == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total": 0, "offset": 0, "limit": walletTxDefaultLimit, "items": []interface{}{},
			})
			return
		}
		offset, limit, q, kind := parseWalletTxListQuery(r)
		if reason := walletTxHistoryDeferReason(cfg); reason != "" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total": 0, "offset": offset, "limit": limit, "items": []interface{}{},
				"deferred": true, "defer_reason": reason,
			})
			return
		}
		if cfg.WalletTxs != nil {
			if page, ok := cfg.WalletTxs.ListPage(offset, limit, q, kind); ok && page.Total > 0 {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"total": page.Total, "offset": page.Offset, "limit": page.Limit, "items": page.Items,
				})
				return
			}
		}
		if strings.EqualFold(strings.TrimSpace(kind), "all") && walletHasScannedIndex(cfg.Wallet) {
			if total, items, ok := walletTxPageMergedAll(cfg, offset, limit, q); ok {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"total": total, "offset": offset, "limit": limit, "items": items,
				})
				return
			}
		}
		if walletTxHistoryUsesScannedSendFastPath(kind) {
			if total, items, ok := walletTxPageFromScannedSend(cfg, offset, limit, q, kind); ok {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"total": total, "offset": offset, "limit": limit, "items": items,
				})
				return
			}
		}
		if walletTxHistoryUsesUtxoFastPath(kind) {
			if total, items, ok := walletTxPageFromUtxoCache(cfg, offset, limit, q, kind); ok {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"total": total, "offset": offset, "limit": limit, "items": items,
				})
				return
			}
		}
		if cfg.WalletTxs == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total": 0, "offset": offset, "limit": limit, "items": []interface{}{},
			})
			return
		}
		txs := cfg.WalletTxs.List()
		if txs == nil {
			txs = []interface{}{}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total": len(txs), "offset": 0, "limit": len(txs), "items": txs,
		})
	})
	mux.HandleFunc("/api/wallet/txs.csv", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if reason := walletTxHistoryDeferReason(cfg); reason != "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-DogeGo-Wallet-Defer-Reason", reason)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("wallet history deferred: " + reason))
			return
		}
		var txs []interface{}
		_, _, q, kind := parseWalletTxListQuery(r)
		if cfg.Wallet != nil {
			if cfg.WalletTxs != nil {
				if page, ok := cfg.WalletTxs.ListPage(0, 0, q, kind); ok {
					txs = page.Items
				} else {
					txs = cfg.WalletTxs.List()
				}
			}
			if txs == nil && strings.EqualFold(strings.TrimSpace(kind), "all") && walletHasScannedIndex(cfg.Wallet) {
				if _, items, ok := walletTxPageMergedAll(cfg, 0, 0, q); ok {
					txs = items
				}
			}
			if txs == nil && walletTxHistoryUsesScannedSendFastPath(kind) {
				if _, items, ok := walletTxPageFromScannedSend(cfg, 0, 0, q, kind); ok {
					txs = items
				}
			}
			if txs == nil && walletTxHistoryUsesUtxoFastPath(kind) {
				if _, items, ok := walletTxPageFromUtxoCache(cfg, 0, 0, q, kind); ok {
					txs = items
				}
			}
		}
		if txs == nil {
			txs = []interface{}{}
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="dogego-wallet-history.csv"`)
		_, _ = w.Write(WalletTransactionsCSV(txs))
	})
}

func registerWalletUtxosRoute(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	mux.HandleFunc("/api/wallet/utxos", func(w http.ResponseWriter, r *http.Request) {
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
		if cfg.Wallet == nil {
			_ = json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		if utxos, ok := walletListUnspentFromUtxoCache(cfg); ok {
			_ = json.NewEncoder(w).Encode(utxos)
			return
		}
		utxos := walletListUnspent(cfg)
		if utxos == nil {
			utxos = []interface{}{}
		}
		_ = json.NewEncoder(w).Encode(utxos)
	})
}
