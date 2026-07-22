// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"math"
	"strings"
)

// fundFeePerKBFromOptions resolves fee rate (koinu/kB) from fundrawtransaction options.
// Explicit fee_rate / feeRate wins; else conf_target + estimate_mode; else 0 (caller applies wallet/min relay).
func fundFeePerKBFromOptions(paths *DataPaths, opts map[string]json.RawMessage) (uint64, int, string) {
	if opts == nil {
		return 0, 0, ""
	}
	for _, key := range []string{"fee_rate", "feeRate"} {
		if v, ok := opts[key]; ok {
			rate, code, msg := parseFeeRateDOGEPerKB(v, "fundrawtransaction")
			if code != 0 {
				return 0, code, msg
			}
			return uint64(math.Round(rate * 1e8)), 0, ""
		}
	}
	confTarget := 0
	if v, ok := opts["conf_target"]; ok {
		var n json.Number
		if err := json.Unmarshal(v, &n); err != nil {
			return 0, -8, "fundrawtransaction: conf_target must be a number"
		}
		ct, err := n.Int64()
		if err != nil || ct < 1 || ct > 1008 {
			return 0, -8, "fundrawtransaction: conf_target out of range"
		}
		confTarget = int(ct)
	}
	if confTarget == 0 {
		return 0, 0, ""
	}
	conservative := true
	if v, ok := opts["estimate_mode"]; ok {
		var mode string
		if err := json.Unmarshal(v, &mode); err != nil {
			return 0, -8, "fundrawtransaction: estimate_mode must be a string"
		}
		switch strings.ToLower(strings.TrimSpace(mode)) {
		case "economical", "eco":
			conservative = false
		case "conservative", "conservative_mode", "":
			conservative = true
		default:
			return 0, -8, "fundrawtransaction: estimate_mode must be conservative or economical"
		}
	}
	rate, _, _ := smartFeeKoinuPerKB(paths, confTarget, conservative)
	if rate > 0 {
		return rate, 0, ""
	}
	return 0, 0, ""
}
