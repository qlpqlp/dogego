// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"dogego/store"
)

// execLoadTxOutSet loads a dumptxoutset JSON-lines snapshot into the in-memory UTXO cache (Core loadtxoutset subset).
func execLoadTxOutSet(j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if j == nil {
		return nil, -1, "loadtxoutset: header journal not available"
	}
	if paths == nil || paths.Utxo == nil {
		return nil, -1, "loadtxoutset: UTXO cache not available"
	}
	if len(params) < 1 {
		return nil, -8, "loadtxoutset: path required"
	}
	var path string
	if err := json.Unmarshal(params[0], &path); err != nil {
		return nil, -8, "loadtxoutset: path must be a string"
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, -8, "loadtxoutset: path required"
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, -1, err.Error()
	}
	chainTip, _, _ := activeChainFromJournal(j, raw, paths)
	n, err := paths.Utxo.LoadFromJSONLFile(abs, chainTip)
	if err != nil {
		return nil, -1, "loadtxoutset: " + err.Error()
	}
	best, err := blockHashHexAt(j, chainTip)
	if err != nil {
		return nil, -1, "loadtxoutset: " + err.Error()
	}
	hashSer := paths.Utxo.SerializedHash()
	if hj, ok := j.(*store.HeaderJournal); ok {
		hashSer = paths.Utxo.SerializedHashAtTip(hj)
	}
	return map[string]interface{}{
		"coins_loaded":    n,
		"base_height":     chainTip,
		"base_hash":       best,
		"path":            abs,
		"txoutset_hash":   hashSer,
		"dogego_format":   "jsonl (from dumptxoutset; not Core serialized chainstate)",
	}, 0, ""
}
