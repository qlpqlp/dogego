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

	"dogego/applog"
	"dogego/store"
)

func parseOneBlockHashParam(params []json.RawMessage, method string) (string, int, string) {
	if len(params) < 1 {
		return "", -8, method + ": blockhash required"
	}
	var s string
	if err := json.Unmarshal(params[0], &s); err != nil {
		return "", -8, method + ": blockhash must be a string"
	}
	s = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(s), "0x"))
	if len(s) != 64 {
		return "", -8, method + ": blockhash must be 64 hex characters"
	}
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return "", -8, method + ": blockhash must be hex"
	}
	return s, 0, ""
}

func parseOneTxidParam(params []json.RawMessage, method string) (string, int, string) {
	return parseOneBlockHashParam(params, method)
}

// execPreciousBlock marks a block as preferred for equal-work reorgs (Core preciousblock).
func execPreciousBlock(j HeaderJournal, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	hash, code, msg := parseOneBlockHashParam(params, "preciousblock")
	if code != 0 {
		return nil, code, msg
	}
	if paths == nil || paths.MarkPreciousBlock == nil {
		return nil, -1, "preciousblock: chain control not available"
	}
	if err := paths.MarkPreciousBlock(hash); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, -5, "Block not found"
		}
		return nil, -1, "preciousblock: " + err.Error()
	}
	return nil, 0, ""
}

// execInvalidateBlock disconnects the chain before hash and marks it invalid (Core invalidateblock).
func execInvalidateBlock(j HeaderJournal, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	hash, code, msg := parseOneBlockHashParam(params, "invalidateblock")
	if code != 0 {
		return nil, code, msg
	}
	if paths == nil || paths.InvalidateBlock == nil {
		return nil, -1, "invalidateblock: chain control not available"
	}
	if err := paths.InvalidateBlock(hash); err != nil {
		if strings.Contains(err.Error(), "genesis") {
			return nil, -8, err.Error()
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, -5, "Block not found"
		}
		return nil, -1, "invalidateblock: " + err.Error()
	}
	return nil, 0, ""
}

// execReconsiderBlock removes a block from the invalid set (Core reconsiderblock).
func execReconsiderBlock(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	hash, code, msg := parseOneBlockHashParam(params, "reconsiderblock")
	if code != 0 {
		return nil, code, msg
	}
	if paths == nil || paths.ReconsiderBlock == nil {
		return nil, -1, "reconsiderblock: chain control not available"
	}
	if err := paths.ReconsiderBlock(hash); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, -5, "Block not found"
		}
		return nil, -1, "reconsiderblock: " + err.Error()
	}
	return nil, 0, ""
}

// execPruneBlockchain removes raw blocks below height (headers kept; Core index pruned when wired).
func execPruneBlockchain(j HeaderJournal, raw *store.RawBlockStore, txIndex *store.TxIndex, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var h float64
	if err := json.Unmarshal(params[0], &h); err != nil {
		return nil, -8, "pruneblockchain: height must be a number"
	}
	if h < 0 || h != float64(int64(h)) {
		return nil, -8, "pruneblockchain: negative or non-integer block height"
	}
	target := int64(h)
	chainTip, _, _ := activeChainFromJournal(j, raw, paths)
	if target > 1_000_000_000 {
		hj, ok := j.(*store.HeaderJournal)
		if !ok || hj == nil {
			return nil, -1, "pruneblockchain: header journal not available"
		}
		var found int64 = -1
		for height := int64(0); height <= chainTip; height++ {
			h80, err := hj.ReadHeaderAt(height)
			if err != nil {
				return nil, -8, "pruneblockchain: could not find block with at least the specified timestamp"
			}
			if headerTime(h80) >= uint32(target)-7200 {
				found = height
				break
			}
		}
		if found < 0 {
			return nil, -8, "pruneblockchain: could not find block with at least the specified timestamp"
		}
		target = found
	}
	if target > chainTip {
		target = chainTip
	}
	hj, ok := j.(*store.HeaderJournal)
	if !ok || hj == nil {
		return nil, -1, "pruneblockchain: header journal not available"
	}
	if raw == nil {
		return nil, -1, "pruneblockchain: raw block store not available"
	}
	last, removed, err := store.PruneRawBlocksBelowHeight(hj, raw, txIndex, target)
	if err != nil {
		return nil, -1, "pruneblockchain: " + err.Error()
	}
	if removed == 0 {
		return int64(0), 0, ""
	}
	if paths != nil && paths.ChainDataDir != "" {
		if err := store.SavePruneMarker(paths.ChainDataDir, last); err != nil {
			applog.Line("block", "prune marker save: "+err.Error())
		}
	}
	return last, 0, ""
}

func headerTime(h80 []byte) uint32 {
	if len(h80) < 72 {
		return 0
	}
	return binary.LittleEndian.Uint32(h80[68:72])
}
