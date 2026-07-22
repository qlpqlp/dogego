// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
)

// parseMaxFeeRateDOGEPerKB reads params[idx] as max fee rate in DOGE/kB (Core uses BTC/kB).
func parseMaxFeeRateDOGEPerKB(params []json.RawMessage, idx int, method string) (float64, string) {
	if len(params) <= idx || strings.TrimSpace(string(params[idx])) == "null" {
		return 0, ""
	}
	var rate float64
	if err := json.Unmarshal(params[idx], &rate); err != nil || rate < 0 {
		return 0, method + ": invalid maxfeerate"
	}
	return rate, ""
}
