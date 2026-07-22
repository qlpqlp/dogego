// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"

	"dogego/mempool"
)

// execSetMempoolPaused toggles whether new transactions are accepted (Core-style operator control).
func execSetMempoolPaused(pool *mempool.Pool, params []json.RawMessage) (bool, int, string) {
	if pool == nil {
		return false, -18, "setmempoolpaused: mempool not available"
	}
	if len(params) != 1 {
		return false, -8, "setmempoolpaused: paused (boolean) required"
	}
	var paused bool
	if err := json.Unmarshal(params[0], &paused); err != nil {
		var v float64
		if err2 := json.Unmarshal(params[0], &v); err2 != nil {
			return false, -8, "setmempoolpaused: paused must be boolean"
		}
		paused = v != 0
	}
	pool.SetPaused(paused)
	return true, 0, ""
}
