// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "dogego/store"

// coinstatsIndexEntry matches Core's getindexinfo "coinstatsindex" object shape.
// DogeGo has no separate coinstats LevelDB index; when the UTXO cache is at chainActive tip,
// hash_serialized mirrors gettxoutsetinfo (Core GetUTXOStats algorithm).
func coinstatsIndexEntry(j HeaderJournal, raw *store.RawBlockStore, utxo *store.UtxoCache, paths *DataPaths) map[string]interface{} {
	blocks, _, _ := activeChainFromJournal(j, raw, paths)
	entry := map[string]interface{}{
		"synced":            false,
		"best_block_height": blocks,
		"dogego_note":       "coinstatsindex not built; use gettxoutsetinfo for UTXO set hash",
	}
	if j == nil || utxo == nil {
		return entry
	}
	if utxo.TipHeight() != blocks {
		return entry
	}
	entry["synced"] = true
	entry["dogego_note"] = "No coinstats index file; hash_serialized from in-memory UTXO cache at chainActive tip (same as gettxoutsetinfo)."
	if hj, ok := j.(*store.HeaderJournal); ok {
		entry["hash_serialized"] = utxo.SerializedHashAtTip(hj)
	} else {
		entry["hash_serialized"] = utxo.SerializedHash()
	}
	return entry
}

// execGetIndexInfo returns a Core-shaped summary of local indexes (native indexes/tx).
func execGetIndexInfo(txIndex *store.TxIndex, filters *store.BlockFilterIndex, j HeaderJournal, raw *store.RawBlockStore, utxo *store.UtxoCache, paths *DataPaths) map[string]interface{} {
	out := map[string]interface{}{
		"coinstatsindex": coinstatsIndexEntry(j, raw, utxo, paths),
	}
	blocks, headerTip, contiguousH := activeChainFromJournal(j, raw, paths)
	wantBlocks := int(blocks) + 1
	if wantBlocks < 1 {
		wantBlocks = 0
	}
	headerHeights := int(headerTip) + 1
	rawCount := 0
	if raw != nil {
		if c, err := raw.Count(); err == nil {
			rawCount = c
		}
	}
	haveContiguous := int(contiguousH) + 1
	if contiguousH < 0 {
		haveContiguous = 0
	}
	missing := wantBlocks - haveContiguous
	if missing < 0 {
		missing = 0
	}
	out["dogego_rawblocks"] = map[string]interface{}{
		"stored":               rawCount,
		"contiguous_through":   contiguousH,
		"header_heights":       headerHeights,
		"missing_from_genesis": missing,
	}
	if txIndex == nil {
		out["dogego_note"] = "tx index is not available for this RPC session (SPV, no_tx_index, or indexer failed to open)"
		return out
	}
	n, sz, err := txIndex.Stats()
	if err != nil {
		out["txindex"] = map[string]interface{}{
			"dogego_error": err.Error(),
		}
		out["dogego_note"] = "could not read indexes/tx directory"
		return out
	}
	legacy, v2, _ := txIndex.FormatStats()
	out["txindex"] = map[string]interface{}{
		"synced":                 haveContiguous >= wantBlocks && wantBlocks > 0,
		"best_block_height":      blocks,
		"size_on_disk":           sz,
		"dogego_tx_files":        n,
		"dogego_legacy_files":    legacy,
		"dogego_v2_files":        v2,
		"dogego_index_path":      txIndex.RootDir(),
		"dogego_note":            "flat per-txid files under indexes/tx (v2 embeds tx raw; legacy 36-byte entries upgraded in background); synced when rawblocks cover chainActive tip",
	}
	if filters != nil {
		fn, _ := filters.Count()
		out["basic block filter"] = map[string]interface{}{
			"synced":              fn >= haveContiguous && haveContiguous > 0,
			"best_block_height":   blocks,
			"dogego_filter_files": fn,
			"dogego_index_path":   filters.Dir(),
			"dogego_note":         "BIP158 basic filters under filters/basic/ (built on block store + tx index)",
		}
	}
	return out
}
