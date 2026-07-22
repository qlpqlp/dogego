// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"sync"
)

const defaultSyncUtxoMaxBlocks = 8

var syncUtxoRPCGate struct {
	mu      sync.Mutex
	running bool
}

// SyncUtxoRPCInFlight reports whether a background syncutxo goroutine is active.
func SyncUtxoRPCInFlight() bool {
	syncUtxoRPCGate.mu.Lock()
	defer syncUtxoRPCGate.mu.Unlock()
	return syncUtxoRPCGate.running
}

// execSyncUtxo advances chainActive through stored bodies (bounded; safe during IBD).
func execSyncUtxo(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if paths == nil || paths.SyncUtxo == nil {
		return nil, -18, "syncutxo: UTXO sync not available"
	}
	maxBlocks := defaultSyncUtxoMaxBlocks
	if len(params) > 0 && len(params[0]) > 0 {
		var n int
		if err := json.Unmarshal(params[0], &n); err == nil && n > 0 {
			maxBlocks = n
			if maxBlocks > 128 {
				maxBlocks = 128
			}
		}
	}
	before := int64(-1)
	if paths.Utxo != nil {
		before = paths.Utxo.TipHeight()
	}
	run := func() error {
		if paths.SyncUtxoBounded != nil {
			return paths.SyncUtxoBounded(maxBlocks)
		}
		return paths.SyncUtxo()
	}
	syncUtxoRPCGate.mu.Lock()
	if syncUtxoRPCGate.running {
		syncUtxoRPCGate.mu.Unlock()
		return map[string]interface{}{
			"success":             true,
			"height_before":       before,
			"height_after":        before,
			"blocks_applied":      int64(0),
			"already_in_flight":   true,
			"dogego_syncutxo_async": true,
		}, 0, ""
	}
	syncUtxoRPCGate.running = true
	syncUtxoRPCGate.mu.Unlock()
	go func() {
		defer func() {
			syncUtxoRPCGate.mu.Lock()
			syncUtxoRPCGate.running = false
			syncUtxoRPCGate.mu.Unlock()
		}()
		_ = run()
	}()
	return map[string]interface{}{
		"success":               true,
		"height_before":         before,
		"max_blocks":            maxBlocks,
		"dogego_syncutxo_async": true,
		"note":                  "connect replay running in background; poll getblockchaininfo blocks or dogego_connect_blocks_per_minute",
	}, 0, ""
}
