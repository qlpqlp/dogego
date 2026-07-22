// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/store"
)

// tryLoadWalletUtxoCacheRows returns persisted wallet-filtered UTXO rows when chain tip and scripts match.
func tryLoadWalletUtxoCacheRows(paths *DataPaths, scripts [][]byte) ([]store.UtxoDumpRow, bool) {
	if paths == nil || paths.Utxo == nil || paths.ChainDataDir == "" || len(scripts) == 0 {
		return nil, false
	}
	tip := paths.Utxo.TipHeight()
	if tip < 0 {
		return nil, false
	}
	key := store.WalletScriptsKey(scripts)
	path := store.WalletUtxoCachePath(paths.ChainDataDir)
	rows, ok := store.LoadWalletUtxoCache(path, tip, key)
	return rows, ok
}

// persistWalletUtxoCacheRows saves filtered wallet UTXO rows for restart-fast queries.
func persistWalletUtxoCacheRows(paths *DataPaths, scripts [][]byte, rows []store.UtxoDumpRow) {
	if paths == nil || paths.Utxo == nil || paths.ChainDataDir == "" || len(scripts) == 0 {
		return
	}
	tip := paths.Utxo.TipHeight()
	if tip < 0 {
		return
	}
	key := store.WalletScriptsKey(scripts)
	path := store.WalletUtxoCachePath(paths.ChainDataDir)
	_ = store.SaveWalletUtxoCache(path, tip, key, rows)
}
