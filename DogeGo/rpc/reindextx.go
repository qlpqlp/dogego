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

// execReindexTx rebuilds indexes/tx from every raw block (Core maintenance analogue to dogego indexer reindex-tx).
// Optional param: clear (boolean) - remove existing tx index files before rebuild (default false).
func execReindexTx(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if paths == nil || strings.TrimSpace(paths.ChainDataDir) == "" {
		return nil, -1, "reindextx: chain data directory not available"
	}
	clear := false
	if len(params) > 0 {
		if err := json.Unmarshal(params[0], &clear); err != nil {
			return nil, -8, "reindextx: clear must be boolean"
		}
	}
	rep, err := store.ReindexTxFromRawBlocks(paths.ChainDataDir, clear)
	if err != nil {
		return nil, -1, "reindextx: " + err.Error()
	}
	return map[string]interface{}{
		"blocks_indexed":  rep.BlocksIndexed,
		"tx_files":        rep.TxFiles,
		"addr_recv_files": rep.AddrRecvFiles,
		"addr_spend_files": rep.AddrSpendFiles,
		"outspend_files":  rep.OutSpendFiles,
		"skipped":         rep.Skipped,
		"dogego_note":     "rebuilt indexes/tx and indexes/addr from rawblocks (run once after upgrade; clear=true recommended)",
	}, 0, ""
}
