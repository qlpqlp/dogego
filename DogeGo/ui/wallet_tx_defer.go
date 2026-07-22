// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"

	"dogego/wallet"
)

// walletTxHistoryDeferReason mirrors ui/static/app.js shouldDeferHeavyWalletAPI for GET /api/wallet/txs.
// Returns empty string when history may load.
func walletTxHistoryDeferReason(cfg StartConfig) string {
	var ibd bool
	var connectLag int64
	if cfg.RPCInvoke != nil {
		res := cfg.RPCInvoke("getblockchaininfo", nil)
		if info, ok := res["result"].(map[string]interface{}); ok {
			ibd, _ = info["initialblockdownload"].(bool)
			if lag, ok := intFromAny(info["dogego_connect_lag"]); ok {
				connectLag = lag
			}
		}
	}
	env := walletDeferEnv(cfg)
	scanning, utxoWalk, scanPending := walletScanDeferFlags(cfg, env)
	utxoCount := walletScanBuildUtxoCount(cfg, env)
	return wallet.HistoryDeferReason(ibd, connectLag, scanning, utxoWalk, scanPending, utxoCount)
}

func walletDeferEnv(cfg StartConfig) map[string]any {
	env := map[string]any{}
	attachWalletRescanStatus(env, cfg)
	if cfg.Wallet != nil {
		if _, _, utxos, ok := walletBalanceFromUtxoCache(cfg); ok {
			env["utxo_count"] = utxos
			return env
		}
	}
	if cfg.RPCInvoke != nil {
		res := cfg.RPCInvoke("getwalletinfo", nil)
		if info, ok := res["result"].(map[string]interface{}); ok {
			if c, ok := intFromAny(info["spendable_utxo_count"]); ok {
				env["utxo_count"] = int(c)
			}
		}
	}
	return env
}

func walletTxHistoryDeferReasonFromProbeState(invoke func(string, []json.RawMessage) map[string]interface{}, out CoreWalletProbeResult) string {
	var ibd bool
	var connectLag int64
	if invoke != nil {
		res := invoke("getblockchaininfo", nil)
		if info, ok := res["result"].(map[string]interface{}); ok {
			ibd, _ = info["initialblockdownload"].(bool)
			if lag, ok := intFromAny(info["dogego_connect_lag"]); ok {
				connectLag = lag
			}
		}
	}
	utxoWalk := out.WalletListTransactionsUtxoWalk != nil && *out.WalletListTransactionsUtxoWalk
	scanPending := out.WalletListTransactionsScanPending != nil && *out.WalletListTransactionsScanPending
	utxoCount := 0
	if out.SpendableUtxoCount != nil {
		utxoCount = *out.SpendableUtxoCount
	}
	return wallet.HistoryDeferReason(ibd, connectLag, out.WalletScanning, utxoWalk, scanPending, utxoCount)
}

// attachWalletHistoryDeferStatus adds wallet_history_deferred + wallet_history_defer_reason to GET /api/wallet and summary.
func attachWalletHistoryDeferStatus(out map[string]any, cfg StartConfig) {
	if out == nil || cfg.Wallet == nil {
		return
	}
	reason := walletTxHistoryDeferReason(cfg)
	if reason == "" {
		return
	}
	out["wallet_history_deferred"] = true
	out["wallet_history_defer_reason"] = reason
}

func walletScanDeferFlags(cfg StartConfig, env map[string]any) (scanning, utxoWalk, scanPending bool) {
	scanning = env != nil && env["scanning"] == true
	if env != nil {
		utxoWalk = env["wallet_listtransactions_utxo_walk"] == true
		scanPending = env["wallet_listtransactions_scan_pending"] == true
	}
	if cfg.RPCInvoke == nil {
		return scanning, utxoWalk, scanPending
	}
	res := cfg.RPCInvoke("getwalletinfo", nil)
	info, ok := res["result"].(map[string]interface{})
	if !ok {
		return scanning, utxoWalk, scanPending
	}
	if _, ok := info["scanning"]; ok {
		scanning = true
	}
	if p, ok := info["dogego_wallet_listtransactions_scan_pending"].(bool); ok && p {
		scanPending = true
	}
	if w, ok := info["dogego_wallet_listtransactions_utxo_walk"].(bool); ok && w {
		utxoWalk = true
	}
	return scanning, utxoWalk, scanPending
}

func walletScanBuildUtxoCount(cfg StartConfig, env map[string]any) int {
	n := walletEnvUtxoCount(env)
	if cfg.RPCInvoke == nil {
		return n
	}
	res := cfg.RPCInvoke("getwalletinfo", nil)
	info, ok := res["result"].(map[string]interface{})
	if !ok {
		return n
	}
	if c, ok := intFromAny(info["spendable_utxo_count"]); ok && int(c) > n {
		n = int(c)
	}
	return n
}

func walletEnvUtxoCount(env map[string]any) int {
	if env == nil {
		return 0
	}
	if n, ok := intFromAny(env["utxo_count"]); ok {
		return int(n)
	}
	if n, ok := intFromAny(env["spendable_utxo_count"]); ok {
		return int(n)
	}
	return 0
}
