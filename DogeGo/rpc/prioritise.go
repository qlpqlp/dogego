// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
	"strings"

	"dogego/mempool"
)

// execPrioritiseTransaction implements prioritisetransaction (Core mining.cpp).
// fee_delta is applied per-tx and propagated to ancestor/descendant mining scores (Core CTxMemPool::PrioritiseTransaction).
func execPrioritiseTransaction(pool *mempool.Pool, params []json.RawMessage) (bool, int, string) {
	if pool == nil {
		return false, -18, "prioritisetransaction: mempool not available"
	}
	if len(params) != 3 {
		return false, -8, "prioritisetransaction: txid, priority_delta, and fee_delta required"
	}
	var txid string
	if err := json.Unmarshal(params[0], &txid); err != nil {
		return false, -8, "prioritisetransaction: bad txid"
	}
	txid = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(txid), "0x"))
	if len(txid) != 64 {
		return false, -8, "prioritisetransaction: txid must be 64 hex characters"
	}
	for _, c := range txid {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false, -8, "prioritisetransaction: txid must be 64 hex characters"
	}
	var prio float64
	if err := json.Unmarshal(params[1], &prio); err != nil {
		return false, -8, "prioritisetransaction: bad priority_delta"
	}
	feeDelta, err := parseJSONInt64Param(params[2])
	if err != nil {
		return false, -8, "prioritisetransaction: bad fee_delta"
	}
	_ = prio
	if err := pool.AddFeeDelta(txid, feeDelta); err != nil {
		return false, -8, "prioritisetransaction: " + err.Error()
	}
	return true, 0, ""
}

func parseJSONInt64Param(r json.RawMessage) (int64, error) {
	var i int64
	if err := json.Unmarshal(r, &i); err == nil {
		return i, nil
	}
	var f float64
	if err := json.Unmarshal(r, &f); err != nil {
		return 0, err
	}
	if f != float64(int64(f)) {
		return 0, fmt.Errorf("not an integer")
	}
	return int64(f), nil
}
