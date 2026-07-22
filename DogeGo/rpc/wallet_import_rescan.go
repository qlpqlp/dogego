// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/store"
	"dogego/wallet/corewallet"
)

// walletMaybeRefillKeypool extends the HD receive/change keypool when configured (no-op for legacy single-key wallets).
func walletMaybeRefillKeypool(paths *DataPaths) {
	walletKeypoolRefill(paths, 0)
}

// walletKeypoolRefillForPoolProbe tops up the HD keypool after native wallet.dat import when Core pool rows were present.
func walletKeypoolRefillForPoolProbe(paths *DataPaths, probe *corewallet.ProbeResult) {
	if paths == nil || probe == nil || probe.PoolCount == 0 {
		return
	}
	walletKeypoolRefill(paths, corewallet.SuggestedKeypoolRefillSize(probe.PoolKeysUnmatched))
}

func walletKeypoolRefill(paths *DataPaths, newSize int) {
	if paths != nil && paths.WalletKeypoolRefill != nil {
		_ = paths.WalletKeypoolRefill(newSize)
	}
}

// walletRescanAfterImport runs SyncUtxo + block scan when rescan is true (importaddress/importpubkey/importprivkey).
func walletRescanAfterImport(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage, heightIdx int, errPrefix string) (int, string) {
	if paths == nil {
		return 0, ""
	}
	start := int64(0)
	if heightIdx >= 0 && len(params) > heightIdx && strings.TrimSpace(string(params[heightIdx])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[heightIdx], &n); err == nil {
			if hi, err := n.Int64(); err == nil && hi >= 0 {
				start = hi
				if j != nil && hi > ActiveChainBlockHeight(j, raw, paths) {
					return -8, errPrefix + ": Block height out of range"
				}
			}
		}
	}
	if paths.ChainDataDir != "" {
		_ = store.InvalidateWalletUtxoCache(paths.ChainDataDir)
	}
	if paths.SyncUtxo != nil {
		if err := WalletRescanSyncUtxoIfNeeded(paths, j, raw); err != nil {
			return -1, errPrefix + ": " + err.Error()
		}
	}
	if paths.WalletRescanBlocks == nil {
		return 0, ""
	}
	if err := paths.WalletRescanBlocks(start); err != nil {
		return -1, errPrefix + ": rescan failed: " + err.Error()
	}
	return 0, ""
}
