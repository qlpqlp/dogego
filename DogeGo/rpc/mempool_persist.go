// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"fmt"
	"strings"

	"dogego/applog"
	"dogego/chain"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// execSaveMempool dumps the in-memory mempool to dogego_mempool.json (Core savemempool analogue).
func execSaveMempool(pool *mempool.Pool, paths *DataPaths) (interface{}, int, string) {
	if pool == nil {
		return nil, -18, "savemempool: mempool not available"
	}
	if paths == nil || strings.TrimSpace(paths.ChainDataDir) == "" {
		return nil, -1, "savemempool: chain data directory not available"
	}
	path := mempool.PersistPath(paths.ChainDataDir)
	if err := mempool.SavePersisted(path, pool.RawBlobs(), pool.ExportFeeDeltas()); err != nil {
		return nil, -1, "savemempool: " + err.Error()
	}
	applog.Line("mempool", "saved mempool to "+path)
	return true, 0, ""
}

// execLoadMempool reloads transactions from dogego_mempool.json through mempool admission (Core loadmempool analogue).
func execLoadMempool(pool *mempool.Pool, paths *DataPaths, j HeaderJournal, txIndex *store.TxIndex, raw *store.RawBlockStore, net chain.Network) (interface{}, int, string) {
	if pool == nil {
		return nil, -18, "loadmempool: mempool not available"
	}
	if paths == nil || strings.TrimSpace(paths.ChainDataDir) == "" {
		return nil, -1, "loadmempool: chain data directory not available"
	}
	path := mempool.PersistPath(paths.ChainDataDir)
	snap, err := mempool.LoadPersistedSnapshot(path)
	if err != nil {
		return nil, -1, "loadmempool: " + err.Error()
	}
	if len(snap.Transactions) == 0 {
		return map[string]interface{}{
			"loaded":       0,
			"skipped":      0,
			"dogego_note":  "no persisted mempool file or file empty",
		}, 0, ""
	}
	adm := newMempoolAdmission(pool, txIndex, raw, j, paths, net)
	loaded, skipped := 0, 0
	for _, rawTx := range snap.Transactions {
		tx, err := wire.DeserializeTx(rawTx)
		if err != nil {
			skipped++
			continue
		}
		if err := acceptMempoolTxRPC(rawTx, tx, pool, paths, adm); err != nil {
			skipped++
			continue
		}
		loaded++
	}
	pool.RestoreFeeDeltas(snap.FeeDeltas)
	applog.Line("mempool", fmt.Sprintf("loadmempool: loaded %d skipped %d", loaded, skipped))
	return map[string]interface{}{
		"loaded":      loaded,
		"skipped":     skipped,
		"dogego_note": "DogeGo reloads dogego_mempool.json with current relay policy; not Core mempool.dat",
	}, 0, ""
}
