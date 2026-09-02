// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"dogego/consensus"
	"dogego/ui/websecurity"
	"dogego/wallet"
)

// walletSendErrorResponse shapes Send-tab errors with optional fee guidance (Core-shaped codes).
func walletSendErrorResponse(code int, msg string, cfg StartConfig) map[string]interface{} {
	out := map[string]interface{}{
		"error": msg,
		"code":  code,
	}
	if code == -6 {
		if confirmed, immature, _, ok := walletBalanceFromUtxoCache(cfg); ok && immature > 0 && confirmed < 1e-8 {
			hint := fmt.Sprintf(
				"Mining rewards are not spendable yet (coinbase maturity ~240 blocks on testnet). Spendable: %.4f DOGE; immature: %.4f DOGE. Raising the fee rate will not help until coinbases mature.",
				confirmed, immature,
			)
			out["fee_hint"] = hint
			out["immature_only"] = true
			return out
		}
	}
	lower := strings.ToLower(msg)
	feeRelated := code == -6 || code == -4 || code == -25 ||
		strings.Contains(lower, "fee") || strings.Contains(lower, "insufficient")
	if !feeRelated {
		return out
	}
	rate := walletSuggestedFeeRateDOGE(cfg)
	est := estimateWalletSendFeeDOGE(rate)
	out["suggested_fee_rate"] = rate
	out["estimated_fee_doge"] = est
	hint := fmt.Sprintf("Raise the fee rate to about %.6f DOGE/kB (~%.4f DOGE fee) and try again.", rate, est)
	if code == -6 {
		hint = "Not enough for amount + network fee. " + hint
	}
	out["fee_hint"] = hint
	return out
}

func registerWalletSendRoute(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	mux.HandleFunc("/api/wallet/send", func(w http.ResponseWriter, r *http.Request) {
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
		if cfg.ActiveWallet() == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "wallet disabled"})
			return
		}
		if cfg.WalletSend == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "wallet send is not available yet"})
			return
		}
		if locked, msg := walletFileLockedErr(cfg); locked {
			w.WriteHeader(http.StatusBadRequest)
			resp := walletSendErrorResponse(-13, msg, cfg)
			resp["wallet_locked"] = true
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if err := cfg.ActiveWallet().RequireMainnetEncryption(cfg.Network); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(walletSendErrorResponse(-15, err.Error(), cfg))
			return
		}
		var body struct {
			Address               string                   `json:"address"`
			Amount                float64                  `json:"amount"`
			PQTag                 string                   `json:"pq_tag"`
			PQCommit              string                   `json:"pq_commitment"`
			PQMode                string                   `json:"pq_mode"`
			FeeRate               *float64                 `json:"fee_rate"`
			SubtractFeeFromAmount bool                     `json:"subtract_fee_from_amount"`
			Inputs                []map[string]interface{} `json:"inputs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
			return
		}
		dest := strings.TrimSpace(body.Address)
		if dest == "" || body.Amount <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "address and positive amount required"})
			return
		}
		if strings.EqualFold(strings.TrimSpace(body.PQMode), "carrier") {
			if cfg.ActiveWallet() == nil || !cfg.ActiveWallet().PqCarrierEnabled() {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(walletSendErrorResponse(-8, "pq_carrier_enabled is false (enable under Settings → Wallet)", cfg))
				return
			}
			if cfg.RPCInvoke == nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "wallet send is not available yet"})
				return
			}
			addrJ, _ := json.Marshal(dest)
			amtJ, _ := json.Marshal(body.Amount)
			fundOpts := map[string]interface{}{"skip_pq_commitment": true}
			if tag := strings.TrimSpace(strings.ToUpper(body.PQTag)); tag != "" {
				fundOpts["pq_tag"] = tag
			}
			if body.FeeRate != nil && *body.FeeRate > 0 {
				fundOpts["fee_rate"] = *body.FeeRate
			}
			if body.SubtractFeeFromAmount {
				fundOpts["subtractFeeFromAmount"] = true
			}
			if len(body.Inputs) > 0 {
				fundOpts["inputs"] = body.Inputs
			}
			optsJ, _ := json.Marshal(fundOpts)
			res := cfg.RPCInvoke("dogego_sendpqcarrier", []json.RawMessage{addrJ, amtJ, optsJ})
			if errMsg, code := rpcResultErr(res); code != 0 {
				status := http.StatusBadRequest
				if code == -1 {
					status = http.StatusServiceUnavailable
				}
				w.WriteHeader(status)
				resp := walletSendErrorResponse(code, errMsg, cfg)
				if code == -1 || strings.Contains(strings.ToLower(errMsg), "not available") {
					resp["retryable"] = true
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			result, _ := res["result"].(map[string]interface{})
			resp := map[string]interface{}{
				"pq_mode": "carrier",
				"status":  "broadcasting",
			}
			if result != nil {
				if txc, _ := result["txc_txid"].(string); txc != "" {
					resp["txid"] = txc
					resp["txc_txid"] = txc
				}
				if txr, _ := result["txr_txid"].(string); txr != "" {
					resp["txr_txid"] = txr
				}
				if tag, _ := result["tag"].(string); tag != "" {
					resp["pq_tag"] = tag
				}
				if scheme, _ := result["scheme"].(string); scheme != "" {
					resp["pq_scheme"] = scheme
				}
				if commit, _ := result["commitment"].(string); commit != "" {
					resp["pq_commitment"] = commit
				}
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		var fundOpts map[string]interface{}
		if strings.TrimSpace(body.PQCommit) != "" {
			tag := strings.ToUpper(strings.TrimSpace(body.PQTag))
			if tag == "" {
				tag = "FLC1"
			}
			fundOpts = map[string]interface{}{
				"pqcommit": map[string]interface{}{
					"tag":        tag,
					"commitment": strings.TrimSpace(strings.TrimPrefix(strings.ToLower(body.PQCommit), "0x")),
				},
			}
		}
		if body.FeeRate != nil && *body.FeeRate > 0 {
			if fundOpts == nil {
				fundOpts = map[string]interface{}{}
			}
			fundOpts["fee_rate"] = *body.FeeRate
		}
		if body.SubtractFeeFromAmount {
			if fundOpts == nil {
				fundOpts = map[string]interface{}{}
			}
			fundOpts["subtractFeeFromAmount"] = true
		}
		if len(body.Inputs) > 0 {
			if fundOpts == nil {
				fundOpts = map[string]interface{}{}
			}
			fundOpts["inputs"] = body.Inputs
		}
		detail, code, msg := cfg.WalletSend.CallDetailed(dest, body.Amount, fundOpts)
		if code != 0 {
			status := http.StatusBadRequest
			if code == -1 {
				status = http.StatusServiceUnavailable
			}
			w.WriteHeader(status)
			resp := walletSendErrorResponse(code, msg, cfg)
			if code == -1 || strings.Contains(strings.ToLower(msg), "not available") {
				resp["retryable"] = true
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		resp := map[string]interface{}{
			"txid":   detail.Txid,
			"status": detail.Status,
		}
		if detail.Hex != "" {
			resp["hex"] = detail.Hex
			enrichWalletSendUIEntry(resp, detail.Hex)
		}
		if detail.BroadcastError != "" {
			resp["broadcast_error"] = detail.BroadcastError
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func walletSuggestedFeeRateDOGE(cfg StartConfig) float64 {
	if cfg.ActiveWallet() != nil {
		if fee := cfg.ActiveWallet().PayTxFee(); fee > 0 {
			return fee
		}
	}
	if cfg.RPCInvoke != nil {
		if res := cfg.RPCInvoke("estimatesmartfee", []json.RawMessage{json.RawMessage(`6`)}); res != nil {
			if result, ok := res["result"].(map[string]interface{}); ok {
				if rate, ok := result["feerate"].(float64); ok && rate > 0 {
					return rate
				}
			}
		}
	}
	return math.Max(float64(consensus.MinRelayTxFeePerKB())/1e8*2, wallet.DefaultPayTxFeeDOGE)
}

func estimateWalletSendFeeDOGE(feePerKb float64) float64 {
	if feePerKb <= 0 {
		feePerKb = 0.001
	}
	return math.Max(feePerKb*0.25, 0.0001)
}
