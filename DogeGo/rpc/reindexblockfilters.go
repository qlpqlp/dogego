// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"strings"

	"dogego/store"
)

// execReindexBlockFilters rebuilds persisted BIP158 basic filters from raw blocks + tx index.
func execReindexBlockFilters(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, txIndex *store.TxIndex, filters *store.BlockFilterIndex) (interface{}, int, string) {
	if paths == nil || strings.TrimSpace(paths.ChainDataDir) == "" {
		return nil, -1, "reindexblockfilters: chain data directory not available"
	}
	sj, ok := j.(*store.HeaderJournal)
	if !ok || sj == nil {
		return nil, -1, "reindexblockfilters: header journal not available"
	}
	if raw == nil {
		return nil, -1, "reindexblockfilters: raw block store not available"
	}
	if txIndex == nil {
		return nil, -1, "reindexblockfilters: tx index required (disable -no_tx_index)"
	}
	if filters == nil {
		return nil, -1, "reindexblockfilters: block filter index not available"
	}
	indexer := func(hashLE [32]byte, blockRaw []byte) error {
		return IndexBasicBlockFilter(filters, hashLE, blockRaw, j, raw, txIndex)
	}
	rep, err := store.RebuildBlockFiltersFromRaw(sj, raw, filters, indexer)
	if err != nil {
		return nil, -1, "reindexblockfilters: " + err.Error()
	}
	return map[string]interface{}{
		"blocks_indexed": rep.BlocksIndexed,
		"dogego_note":    "rebuilt filters/basic/ from rawblocks (use after filter encoder changes)",
	}, 0, ""
}
