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
)

// WalletRescanUtxoSynced reports whether chainActive already covers contiguous raw bodies (rescan can skip SyncUtxo).
func WalletRescanUtxoSynced(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore) bool {
	if paths == nil || paths.Utxo == nil {
		return false
	}
	tip := paths.Utxo.TipHeight()
	if tip < 0 {
		return false
	}
	if j != nil && raw != nil {
		if hj, ok := j.(*store.HeaderJournal); ok {
			if cont, err := store.ContiguousRawBodyHeight(hj, raw); err == nil && cont >= 0 {
				return tip >= cont
			}
		}
	}
	return tip >= ActiveChainBlockHeight(j, raw, paths)
}

// WalletRescanSyncUtxoIfNeeded runs SyncUtxo only when chainActive lags stored bodies.
func WalletRescanSyncUtxoIfNeeded(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore) error {
	if paths == nil || paths.SyncUtxo == nil || WalletRescanUtxoSynced(paths, j, raw) {
		return nil
	}
	return paths.SyncUtxo()
}

// execRescanWallet refreshes the UTXO cache when needed and scans raw blocks for wallet script activity.
func execRescanWallet(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	startHeight := int64(0)
	if len(params) == 1 && strings.TrimSpace(string(params[0])) != "null" {
		var h json.Number
		if err := json.Unmarshal(params[0], &h); err != nil {
			return nil, -8, "rescan: height must be a number"
		}
		hi, err := h.Int64()
		if err != nil || hi < 0 {
			return nil, -8, "rescan: height out of range"
		}
		startHeight = hi
		if j != nil && hi > ActiveChainBlockHeight(j, raw, paths) {
			return nil, -8, "Block height out of range"
		}
	}
	if paths == nil || rpcWalletDefaultAddress(paths) == "" {
		return nil, -1, "rescan: wallet is not implemented in DogeGo"
	}
	if paths.SyncUtxo != nil {
		if err := WalletRescanSyncUtxoIfNeeded(paths, j, raw); err != nil {
			return nil, -1, "rescan: "+err.Error()
		}
	}
	if paths.WalletRescanBlocks != nil {
		if err := paths.WalletRescanBlocks(startHeight); err != nil {
			return nil, -1, "rescan: "+err.Error()
		}
	}
	return nil, 0, ""
}
