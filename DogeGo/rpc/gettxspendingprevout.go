// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strconv"
	"strings"

	"dogego/mempool"
)

type prevoutRef struct {
	Txid string `json:"txid"`
	Vout uint32 `json:"vout"`
}

// execGetTxSpendingPrevout scans the mempool for txs spending the given outpoints (Core gettxspendingprevout).
func execGetTxSpendingPrevout(pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	refs, code, msg := parsePrevoutRefList(params[0])
	if code != 0 {
		return nil, code, msg
	}
	if pool == nil {
		out := make([]interface{}, len(refs))
		for i, ref := range refs {
			out[i] = map[string]interface{}{
				"txid": ref.Txid,
				"vout": ref.Vout,
			}
		}
		return out, 0, ""
	}
	out := make([]interface{}, 0, len(refs))
	for _, ref := range refs {
		row := map[string]interface{}{
			"txid": ref.Txid,
			"vout": ref.Vout,
		}
		if spender := pool.SpenderOfOutpoint(ref.Txid, ref.Vout); spender != "" {
			row["spendingtxid"] = spender
		}
		out = append(out, row)
	}
	return out, 0, ""
}

func parsePrevoutRefList(raw json.RawMessage) ([]prevoutRef, int, string) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil, -8, "gettxspendingprevout: outputs array required"
	}
	var refs []prevoutRef
	if err := json.Unmarshal(raw, &refs); err == nil {
		return normalizePrevoutRefs(refs)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, -8, "gettxspendingprevout: outputs must be a JSON array"
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, -8, "gettxspendingprevout: outputs array required"
	}
	if err := json.Unmarshal([]byte(s), &refs); err != nil {
		return nil, -8, "gettxspendingprevout: outputs must be a JSON array"
	}
	return normalizePrevoutRefs(refs)
}

func normalizePrevoutRefs(refs []prevoutRef) ([]prevoutRef, int, string) {
	if len(refs) == 0 {
		return nil, -8, "gettxspendingprevout: outputs array required"
	}
	out := make([]prevoutRef, 0, len(refs))
	for i, ref := range refs {
		txid := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(ref.Txid), "0x"))
		if len(txid) != 64 {
			return nil, -8, "gettxspendingprevout: invalid txid in outputs[" + strconv.Itoa(i) + "]"
		}
		for _, c := range txid {
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
				continue
			}
			return nil, -8, "gettxspendingprevout: invalid txid in outputs[" + strconv.Itoa(i) + "]"
		}
		out = append(out, prevoutRef{Txid: txid, Vout: ref.Vout})
	}
	return out, 0, ""
}
