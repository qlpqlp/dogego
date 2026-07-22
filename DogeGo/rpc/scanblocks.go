// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// execScanBlocks returns block hashes that may contain given descriptors (Core scanblocks; basic filters).
func execScanBlocks(chainName string, j HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, filters *store.BlockFilterIndex, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	if j == nil {
		return nil, -1, "scanblocks: header journal not available"
	}
	if filters == nil {
		return nil, -1, "scanblocks: block filter index not available (enable tx index and filters)"
	}
	var action string
	if err := json.Unmarshal(params[0], &action); err != nil {
		return nil, -8, "scanblocks: action must be a string"
	}
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "abort":
		return false, 0, ""
	case "status":
		return nil, 0, ""
	case "start":
		return runScanBlocks(chainName, j, raw, txIx, filters, paths, params[1:])
	default:
		return nil, -8, "scanblocks: unknown action " + action
	}
}

func runScanBlocks(chainName string, j HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, filters *store.BlockFilterIndex, paths *DataPaths, rest []json.RawMessage) (interface{}, int, string) {
	if len(rest) < 1 {
		return nil, -32602, "scanblocks: scanobjects required for start"
	}
	objects, code, msg := parseScanObjectsParam(rest[0])
	if code != 0 {
		return nil, code, msg
	}
	matchers, code, msg := buildScanTxOutMatchers(chainName, objects)
	if code != 0 {
		return nil, code, msg
	}
	startH := int64(0)
	stopH, _, _ := activeChainFromJournal(j, raw, paths)
	if len(rest) > 1 && strings.TrimSpace(string(rest[1])) != "null" {
		var f float64
		if err := json.Unmarshal(rest[1], &f); err != nil || f < 0 || f != float64(int64(f)) {
			return nil, -8, "scanblocks: start_height must be a non-negative integer"
		}
		startH = int64(f)
	}
	if len(rest) > 2 && strings.TrimSpace(string(rest[2])) != "null" {
		var f float64
		if err := json.Unmarshal(rest[2], &f); err != nil || f < 0 || f != float64(int64(f)) {
			return nil, -8, "scanblocks: stop_height must be a non-negative integer"
		}
		stopH = int64(f)
	}
	filterType := "basic"
	if len(rest) > 3 && strings.TrimSpace(string(rest[3])) != "null" {
		if err := json.Unmarshal(rest[3], &filterType); err != nil {
			return nil, -8, "scanblocks: filtertype must be a string"
		}
		if strings.ToLower(strings.TrimSpace(filterType)) != "basic" {
			return nil, -8, "scanblocks: only basic filtertype is supported"
		}
	}
	verifyBlocks := false
	if len(rest) > 4 && strings.TrimSpace(string(rest[4])) != "null" {
		var opts map[string]json.RawMessage
		if err := json.Unmarshal(rest[4], &opts); err == nil {
			if v, ok := opts["filter_false_positives"]; ok {
				verifyBlocks, _, _ = parseRPCBoolOpt(v, false, "scanblocks", "filter_false_positives")
			}
		}
	}
	if startH > stopH {
		return nil, -8, "scanblocks: start_height after stop_height"
	}

	var relevant []string
	seenHash := make(map[string]struct{})
	for h := startH; h <= stopH; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			break
		}
		hashLE := pow.BlockHashLE(h80)
		enc, _, err := filters.Get(hashLE)
		if err != nil {
			continue
		}
		maybe := false
		for _, m := range matchers {
			if scanMatcherMayMatchFilter(hashLE, enc, m) {
				maybe = true
				break
			}
		}
		if !maybe {
			continue
		}
		if verifyBlocks && raw != nil {
			if !blockContainsMatcherScripts(raw, txIx, hashLE, matchers) {
				continue
			}
		}
		display := pow.LEUint256DisplayHex(hashLE[:])
		if _, ok := seenHash[display]; ok {
			continue
		}
		seenHash[display] = struct{}{}
		relevant = append(relevant, display)
	}
	return map[string]interface{}{
		"from_height":      startH,
		"to_height":        stopH,
		"relevant_blocks":  relevant,
		"completed":        true,
	}, 0, ""
}

func scanMatcherMayMatchFilter(hashLE [32]byte, encoded []byte, m scanTxOutMatcher) bool {
	for _, probe := range matcherScripts(m) {
		ok, err := consensus.BasicFilterMayContainScript(hashLE, encoded, probe)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func blockContainsMatcherScripts(raw *store.RawBlockStore, txIx *store.TxIndex, hashLE [32]byte, matchers []scanTxOutMatcher) bool {
	payload, err := raw.Get(hashLE)
	if err != nil {
		return false
	}
	var matched bool
	err = wire.ForEachBlockTx(payload, func(_ uint32, tx *wire.Tx) error {
		if matched {
			return nil
		}
		for _, o := range tx.Vout {
			for _, m := range matchers {
				if m.Match(o.PkScript) {
					matched = true
					return nil
				}
			}
		}
		if txIx == nil {
			return nil
		}
		for _, in := range tx.Vin {
			if consensus.IsNullOutpoint(&in) {
				continue
			}
			id := mempool.TxIDDisplayHex(in.PrevHash)
			_, spk, ok := store.LoadIndexedTxVout(txIx, raw, id, in.PrevIdx)
			if !ok {
				continue
			}
			for _, m := range matchers {
				if m.Match(spk) {
					matched = true
					return nil
				}
			}
		}
		return nil
	})
	return err == nil && matched
}
