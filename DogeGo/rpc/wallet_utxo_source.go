// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "dogego/store"

// walletConfirmedUTXORows returns confirmed UTXO rows for wallet coin selection from the in-memory cache.
func walletConfirmedUTXORows(paths *DataPaths, scripts [][]byte, maxResults int) ([]store.UtxoDumpRow, error) {
	if len(scripts) > 0 {
		if cached, ok := tryLoadWalletUtxoCacheRows(paths, scripts); ok {
			if maxResults > 0 && len(cached) > maxResults {
				return cached[:maxResults], nil
			}
			return cached, nil
		}
	}
	rows, err := walletConfirmedUTXORowsFromUtxo(paths, scripts, maxResults)
	if err != nil {
		return nil, err
	}
	if len(scripts) > 0 && maxResults == 0 && len(rows) > 0 {
		persistWalletUtxoCacheRows(paths, scripts, rows)
	}
	return rows, nil
}

// RefreshWalletUtxoCache rebuilds wallet_utxo_scan.cache.json from the live UTXO set.
func RefreshWalletUtxoCache(paths *DataPaths, scripts [][]byte) {
	rows, err := walletConfirmedUTXORowsFromUtxo(paths, scripts, 0)
	if err != nil {
		return
	}
	persistWalletUtxoCacheRows(paths, scripts, rows)
}

func walletConfirmedUTXORowsFromUtxo(paths *DataPaths, scripts [][]byte, maxResults int) ([]store.UtxoDumpRow, error) {
	if paths == nil || paths.Utxo == nil {
		return nil, nil
	}
	if len(scripts) > 0 {
		set := make(map[string]struct{}, len(scripts))
		for _, s := range scripts {
			set[string(s)] = struct{}{}
		}
		return paths.Utxo.FilterRowsByScriptSet(set, maxResults), nil
	}
	rows := paths.Utxo.DumpRows()
	if maxResults > 0 && len(rows) > maxResults {
		return rows[:maxResults], nil
	}
	return rows, nil
}

// walletLookupOutpoint resolves a confirmed UTXO by RPC txid and vout.
func walletLookupOutpoint(paths *DataPaths, rpcTxid string, vout uint32) (store.UtxoEntry, bool) {
	if paths == nil || paths.Utxo == nil {
		return store.UtxoEntry{}, false
	}
	return paths.Utxo.Lookup(rpcTxid, vout)
}
