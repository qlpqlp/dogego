// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dogego/mempool"
	"dogego/store"
)

// execMempoolExists reports whether a transaction id is in the mempool (Core mempoolexists).
func execMempoolExists(pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	if pool == nil {
		return false, 0, ""
	}
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	txid, code, msg := parseOneTxidParam(params, "mempoolexists")
	if code != 0 {
		return nil, code, msg
	}
	return pool.ContainsTxID(txid), 0, ""
}

// execDumpTxOutSet writes a JSON-lines UTXO snapshot from the in-memory UTXO cache.
func execDumpTxOutSet(j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if j == nil {
		return nil, -1, "dumptxoutset: header journal not available"
	}
	chainTip, _, _ := activeChainFromJournal(j, raw, paths)
	if paths == nil || paths.Utxo == nil || paths.Utxo.TipHeight() != chainTip {
		return nil, -1, "dumptxoutset: UTXO cache not synced to chainActive tip"
	}
	rows := paths.Utxo.DumpRows()
	formatNote := "jsonl from in-memory UTXO cache"
	outPath := ""
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		if err := json.Unmarshal(params[0], &outPath); err != nil {
			return nil, -8, "dumptxoutset: path must be a string"
		}
		outPath = strings.TrimSpace(outPath)
	}
	if outPath == "" {
		if paths.ChainDataDir == "" {
			return nil, -1, "dumptxoutset: chain data directory unknown"
		}
		outPath = filepath.Join(paths.ChainDataDir, fmt.Sprintf("utxo-dump-%d.jsonl", chainTip))
	}
	abs, err := filepath.Abs(outPath)
	if err != nil {
		return nil, -1, err.Error()
	}
	best, err := blockHashHexAt(j, chainTip)
	if err != nil {
		return nil, -1, "dumptxoutset: " + err.Error()
	}
	f, err := os.Create(abs)
	if err != nil {
		return nil, -1, "dumptxoutset: " + err.Error()
	}
	enc := json.NewEncoder(f)
	for _, row := range rows {
		if err := enc.Encode(map[string]interface{}{
			"txid":         row.TxID,
			"vout":         row.Vout,
			"value":        row.Value,
			"height":       row.Height,
			"scriptPubKey": hex.EncodeToString(row.PkScript),
		}); err != nil {
			f.Close()
			os.Remove(abs)
			return nil, -1, err.Error()
		}
	}
	if err := f.Close(); err != nil {
		return nil, -1, err.Error()
	}
	hashSer := paths.Utxo.SerializedHash()
	if hj, ok := j.(*store.HeaderJournal); ok {
		hashSer = paths.Utxo.SerializedHashAtTip(hj)
	}
	return map[string]interface{}{
		"coins_written": len(rows),
		"base_hash":     best,
		"base_height":   chainTip,
		"path":          abs,
		"txoutset_hash": hashSer,
		"nchaintx":      chainTip + 1,
		"dogego_format": formatNote,
	}, 0, ""
}
