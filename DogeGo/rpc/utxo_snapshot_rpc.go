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
	"dogego/store"
)

// execSaveUtxoSnapshot writes utxo.cache from the in-memory UTXO set (operator checkpoint before restart).
func execSaveUtxoSnapshot(utxo *store.UtxoCache, paths *DataPaths) (interface{}, int, string) {
	if utxo == nil {
		return nil, -18, "saveutxosnapshot: UTXO cache not available"
	}
	if paths == nil || strings.TrimSpace(paths.ChainDataDir) == "" {
		return nil, -1, "saveutxosnapshot: chain data directory not available"
	}
	tip := utxo.TipHeight()
	if tip < 0 {
		return nil, -1, "saveutxosnapshot: chainActive empty (no connected blocks)"
	}
	if paths.UtxoBodiesAligned != nil && !paths.UtxoBodiesAligned() {
		cont := int64(-1)
		if paths.ContiguousRawHeight != nil {
			cont = paths.ContiguousRawHeight()
		}
		return nil, -1, fmt.Sprintf("saveutxosnapshot: stored bodies through %d lag UTXO tip %d (wait for body replay)", cont, tip)
	}
	path := store.UtxoSnapshotPath(paths.ChainDataDir)
	started, err := utxo.SaveSnapshotAsync(path)
	if err != nil {
		return nil, -1, "saveutxosnapshot: " + err.Error()
	}
	if !started {
		return map[string]interface{}{
			"success":                    true,
			"height":                     tip,
			"outputs":                    utxo.Count(),
			"path":                       path,
			"dogego_utxo_snapshot":       path,
			"dogego_utxo_snapshot_async": true,
			"already_in_flight":          true,
		}, 0, ""
	}
	applog.Line("utxo", fmt.Sprintf("saveutxosnapshot: writing height %d (%d outputs) in background", tip, utxo.Count()))
	return map[string]interface{}{
		"success":                    true,
		"height":                     tip,
		"outputs":                    utxo.Count(),
		"path":                       path,
		"dogego_utxo_snapshot":       path,
		"dogego_utxo_snapshot_async": true,
	}, 0, ""
}
