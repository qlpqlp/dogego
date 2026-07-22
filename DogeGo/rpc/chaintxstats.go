// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"encoding/json"
	"strings"

	"dogego/pow"
	"dogego/store"
)

// execGetChainTxStats returns Core-shaped chain transaction statistics.
func execGetChainTxStats(j HeaderJournal, raw *store.RawBlockStore, txIndex *store.TxIndex, paths *DataPaths, chainName string, params []json.RawMessage) (interface{}, int, string) {
	if j == nil {
		return nil, -1, "getchaintxstats: header journal not available"
	}
	tip, _, _ := activeChainFromJournal(j, raw, paths)
	window := int64(2016)
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		var blockhash string
		if err := json.Unmarshal(params[0], &blockhash); err != nil {
			return nil, -8, "getchaintxstats: blockhash must be a string"
		}
		blockhash = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(blockhash), "0x"))
		if len(blockhash) != 64 {
			return nil, -8, "getchaintxstats: blockhash must be 64 hex characters"
		}
		h, err := j.HeightByDisplayHash(blockhash)
		if err != nil {
			return nil, -5, "Block not found"
		}
		tip = h
	}
	if len(params) > 1 {
		var n float64
		if err := json.Unmarshal(params[1], &n); err != nil || n < 0 || n != float64(int64(n)) {
			return nil, -8, "getchaintxstats: nblocks must be a non-negative integer"
		}
		window = int64(n)
	}
	if window <= 0 || window > tip+1 {
		window = tip + 1
	}
	start := tip + 1 - window
	if start < 0 {
		start = 0
	}
	var tFirst, tLast uint32
	for h := start; h <= tip; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return nil, -1, err.Error()
		}
		ts := binary.LittleEndian.Uint32(h80[68:72])
		if h == start {
			tFirst = ts
		}
		tLast = ts
	}
	interval := int64(tLast - tFirst)
	if interval < 0 {
		interval = 0
	}
	txcount := int64(0)
	indexed := false
	if txIndex != nil {
		if n, _, err := txIndex.Stats(); err == nil && n > 0 {
			txcount = int64(n)
			indexed = true
		}
	}
	if txcount == 0 {
		txcount = tip + 1 // coinbase-only lower bound
	}
	windowTx := int64(0)
	windowFromRaw := false
	if n, ok := countWindowTxsFromRawBlocks(j, raw, start, tip); ok {
		windowTx = n
		windowFromRaw = true
	} else {
		windowTx = window
		if windowTx > txcount {
			windowTx = txcount
		}
	}
	finalHash := ""
	if h80, err := j.ReadHeaderAt(tip); err == nil {
		finalHash = pow.BlockHashHex(h80)
	}
	note := "txcount from tx index when indexed; window_tx_count from rawblocks in range when stored"
	if !indexed {
		note = "txcount is tip+1 coinbase estimate without tx index; window_tx_count approximate"
	}
	if !windowFromRaw {
		note += "; window_tx_count capped (raw blocks missing for part of window)"
	}
	_ = chainName
	return map[string]interface{}{
		"time":                      int64(tLast),
		"txcount":                   txcount,
		"window_final_block_hash":   finalHash,
		"window_final_block_height": tip,
		"window_block_count":        window,
		"window_tx_count":           windowTx,
		"window_interval":           interval,
		"dogego_note":               note,
	}, 0, ""
}
