// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dogego/applog"
	"dogego/chain"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// execImportMempool loads transactions from a DogeGo mempool JSON dump (Core importmempool subset; not mempool.dat).
func execImportMempool(pool *mempool.Pool, paths *DataPaths, j HeaderJournal, txIndex *store.TxIndex, raw *store.RawBlockStore, net chain.Network, params []json.RawMessage) (interface{}, int, string) {
	if pool == nil {
		return nil, -18, "importmempool: mempool not available"
	}
	if len(params) < 1 || len(params) > 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var filePath string
	if err := json.Unmarshal(params[0], &filePath); err != nil {
		return nil, -8, "importmempool: filepath must be a string"
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, -8, "importmempool: filepath must be a string"
	}
	if !filepath.IsAbs(filePath) {
		if paths == nil || strings.TrimSpace(paths.ChainDataDir) == "" {
			return nil, -8, "importmempool: relative filepath requires chain data directory"
		}
		filePath = filepath.Join(paths.ChainDataDir, filePath)
	}
	if _, err := os.Stat(filePath); err != nil {
		return nil, -8, "importmempool: " + err.Error()
	}
	blobs, err := mempool.LoadPersisted(filePath)
	if err != nil {
		return nil, -1, "importmempool: " + err.Error()
	}
	if len(blobs) == 0 {
		return map[string]interface{}{
			"imported":    0,
			"skipped":     0,
			"dogego_note": "file empty or no transactions",
		}, 0, ""
	}
	adm := newMempoolAdmission(pool, txIndex, raw, j, paths, net)
	imported, skipped := 0, 0
	for _, rawTx := range blobs {
		tx, err := wire.DeserializeTx(rawTx)
		if err != nil {
			skipped++
			continue
		}
		if err := acceptMempoolTxRPC(rawTx, tx, pool, paths, adm); err != nil {
			skipped++
			continue
		}
		imported++
	}
	applog.Line("mempool", "importmempool "+filePath+": imported "+strconv.Itoa(imported)+" skipped "+strconv.Itoa(skipped))
	return map[string]interface{}{
		"imported":    imported,
		"skipped":     skipped,
		"dogego_note": "DogeGo JSON mempool dump; not Dogecoin Core mempool.dat",
	}, 0, ""
}
