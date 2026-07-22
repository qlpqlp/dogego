// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"dogego/pow"
	"dogego/store"
)

func parseBasicFilterTypeParam(params []json.RawMessage) (string, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return "", -32602, "Wrong number of arguments"
	}
	filterType := "basic"
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if err := json.Unmarshal(params[1], &filterType); err != nil {
			return "", -8, "filtertype must be a string"
		}
		filterType = strings.ToLower(strings.TrimSpace(filterType))
	}
	if filterType != "basic" {
		return "", -8, "only basic filtertype is supported"
	}
	return filterType, 0, ""
}

type blockFilterResult struct {
	encoded []byte
	header  [32]byte
	height  int64
}

func loadBasicBlockFilter(j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex, filters *store.BlockFilterIndex, params []json.RawMessage) (blockFilterResult, int, string) {
	var empty blockFilterResult
	if raw == nil {
		return empty, -18, "raw block store not available"
	}
	if ix == nil {
		return empty, -18, "tx index required for basic filter prevouts"
	}
	if _, code, msg := parseBasicFilterTypeParam(params); code != 0 {
		return empty, code, msg
	}
	hashLE, height, err := resolveBlockLocation(j, params)
	if err != nil {
		return empty, -8, err.Error()
	}
	if filters != nil && filters.Has(hashLE) {
		encoded, hdr, err := filters.Get(hashLE)
		if err == nil {
			return blockFilterResult{encoded: encoded, header: copy32(hdr), height: height}, 0, ""
		}
	}
	payload, err := raw.Get(hashLE)
	if err != nil {
		return empty, -5, "Block not found"
	}
	var prevHeader [32]byte
	if height > 0 {
		h80, err := j.ReadHeaderAt(height - 1)
		if err == nil {
			prevHash := pow.BlockHashLE(h80)
			if filters != nil {
				if _, hdr, err := filters.Get(prevHash); err == nil {
					copy(prevHeader[:], hdr)
				}
			}
			if prevHeader == [32]byte{} {
				if prevPayload, err := raw.Get(prevHash); err == nil {
					if enc, hdr, err := BuildBasicBlockFilter(prevHash, prevPayload, j, raw, ix, [32]byte{}); err == nil {
						prevHeader = hdr
						_ = enc
					}
				}
			}
		}
	}
	encoded, header, err := BuildBasicBlockFilter(hashLE, payload, j, raw, ix, prevHeader)
	if err != nil {
		return empty, -8, err.Error()
	}
	if filters != nil {
		_ = filters.Put(hashLE, encoded, header[:])
	}
	return blockFilterResult{encoded: encoded, header: header, height: height}, 0, ""
}

// execGetBlockFilter returns a BIP158 basic filter for a block (Core getblockfilter subset).
func execGetBlockFilter(j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex, filters *store.BlockFilterIndex, params []json.RawMessage) (interface{}, int, string) {
	res, code, msg := loadBasicBlockFilter(j, raw, ix, filters, params)
	if code != 0 {
		if msg != "" && !strings.HasPrefix(msg, "getblockfilter:") {
			msg = "getblockfilter: " + msg
		}
		return nil, code, msg
	}
	return map[string]interface{}{
		"filter": hex.EncodeToString(res.encoded),
		"header": displayHashHex(res.header),
		"height": res.height,
	}, 0, ""
}

// execGetBlockFilterHeader returns the BIP158 filter header for a block (Bitcoin Core getblockfilterheader).
func execGetBlockFilterHeader(j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex, filters *store.BlockFilterIndex, params []json.RawMessage) (interface{}, int, string) {
	res, code, msg := loadBasicBlockFilter(j, raw, ix, filters, params)
	if code != 0 {
		if msg != "" && !strings.HasPrefix(msg, "getblockfilterheader:") {
			msg = "getblockfilterheader: " + msg
		}
		return nil, code, msg
	}
	return map[string]interface{}{
		"header": displayHashHex(res.header),
	}, 0, ""
}

func copy32(b []byte) [32]byte {
	var out [32]byte
	if len(b) >= 32 {
		copy(out[:], b[:32])
	}
	return out
}

func displayHashHex(h [32]byte) string {
	return displayHashHexBytes(h[:])
}

func displayHashHexBytes(b []byte) string {
	if len(b) != 32 {
		return hex.EncodeToString(b)
	}
	out := make([]byte, 32)
	for i := 0; i < 32; i++ {
		out[i] = b[31-i]
	}
	return hex.EncodeToString(out)
}
