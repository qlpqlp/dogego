// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/store"
	"dogego/wallet"
)

func mergeWalletHistoryDeferReason(info map[string]interface{}, paths *DataPaths, j HeaderJournal, chainName string, raw *store.RawBlockStore) {
	if info == nil {
		return
	}
	reason := walletHistoryDeferReasonRPC(info, paths, j, chainName, raw)
	if reason == "" {
		return
	}
	info["dogego_wallet_history_deferred"] = true
	info["dogego_wallet_history_defer_reason"] = reason
}

func walletHistoryDeferReasonRPC(info map[string]interface{}, paths *DataPaths, j HeaderJournal, chainName string, raw *store.RawBlockStore) string {
	var ibd bool
	if j != nil {
		sync := computeChainIBDState(j, chainName, raw, paths)
		ibd = sync.ibd
	}
	connectLag := rawSyncConnectLag(paths)
	scanning := false
	if _, ok := info["scanning"]; ok {
		scanning = true
	}
	utxoWalk, _ := info["dogego_wallet_listtransactions_utxo_walk"].(bool)
	scanPending, _ := info["dogego_wallet_listtransactions_scan_pending"].(bool)
	utxoCount := 0
	switch n := info["spendable_utxo_count"].(type) {
	case int:
		utxoCount = n
	case int64:
		utxoCount = int(n)
	}
	return wallet.HistoryDeferReason(ibd, connectLag, scanning, utxoWalk, scanPending, utxoCount)
}

func rawSyncConnectLag(paths *DataPaths) int64 {
	if paths == nil || paths.RawSyncProgress == nil {
		return 0
	}
	prog := paths.RawSyncProgress()
	if prog == nil {
		return 0
	}
	switch v := prog["connect_lag"].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}
