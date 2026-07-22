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

// execUpgradeTxIndex upgrades legacy 36-byte indexes/tx entries to v2 (embedded tx raw).
// Optional param: max_files (integer, default 10000). Use dogego indexer reindex-tx for full rebuild.
func execUpgradeTxIndex(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if paths == nil || strings.TrimSpace(paths.ChainDataDir) == "" {
		return nil, -1, "upgradetxindex: chain data directory not available"
	}
	maxFiles := 10000
	if len(params) > 0 {
		var n float64
		if err := json.Unmarshal(params[0], &n); err != nil {
			return nil, -8, "upgradetxindex: max_files must be a non-negative integer"
		}
		if n < 0 || n != float64(int64(n)) {
			return nil, -8, "upgradetxindex: max_files must be a non-negative integer"
		}
		maxFiles = int(n)
	}
	upgraded, legacy, err := store.UpgradeLegacyTxIndexBatch(paths.ChainDataDir, maxFiles)
	if err != nil {
		return nil, -1, "upgradetxindex: " + err.Error()
	}
	legacyAfter := legacy
	if txIx, err := store.OpenTxIndex(paths.ChainDataDir); err == nil {
		if l, v2, err := txIx.FormatStats(); err == nil {
			legacyAfter = l
			return map[string]interface{}{
				"upgraded":              upgraded,
				"legacy_remaining":      legacyAfter,
				"dogego_v2_files":       v2,
				"dogego_legacy_files":   l,
				"dogego_note":           "legacy entries embed serialized tx for Core-style getrawtransaction/gettxout fast path",
			}, 0, ""
		}
	}
	return map[string]interface{}{
		"upgraded":         upgraded,
		"legacy_remaining": legacyAfter,
		"dogego_note":      "legacy entries embed serialized tx for Core-style getrawtransaction/gettxout fast path",
	}, 0, ""
}
