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

type listUnspentQueryOpts struct {
	minimumAmount    float64
	maximumAmount    float64
	minimumSumAmount float64
	maximumCount     int
}

func parseListUnspentQueryOptions(raw json.RawMessage) (listUnspentQueryOpts, int, string) {
	var out listUnspentQueryOpts
	if code, msg := validateListUnspentQueryOptions(raw); code != 0 {
		return out, code, msg
	}
	var opts map[string]json.RawMessage
	_ = json.Unmarshal(raw, &opts)
	for k, elem := range opts {
		switch k {
		case "minimumAmount":
			out.minimumAmount = parseQueryOptionDOGE(elem)
		case "maximumAmount":
			out.maximumAmount = parseQueryOptionDOGE(elem)
		case "minimumSumAmount":
			out.minimumSumAmount = parseQueryOptionDOGE(elem)
		case "maximumCount":
			out.maximumCount = int(parseQueryOptionInt(elem))
		}
	}
	return out, 0, ""
}

func parseQueryOptionDOGE(raw json.RawMessage) float64 {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	return 0
}

func parseQueryOptionInt(raw json.RawMessage) int64 {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int64(f)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n := json.Number(strings.TrimSpace(s))
		hi, _ := n.Int64()
		return hi
	}
	return 0
}

func filterListUnspentMatches(matches []walletUtxoMatch, opts listUnspentQueryOpts) []walletUtxoMatch {
	if opts.minimumAmount <= 0 && opts.maximumAmount <= 0 && opts.minimumSumAmount <= 0 && opts.maximumCount <= 0 {
		return matches
	}
	minKoinu := int64(opts.minimumAmount * 1e8)
	maxKoinu := int64(0)
	if opts.maximumAmount > 0 {
		maxKoinu = int64(opts.maximumAmount * 1e8)
	}
	var filtered []walletUtxoMatch
	var sum int64
	for _, m := range matches {
		if minKoinu > 0 && m.row.Value < minKoinu {
			continue
		}
		if maxKoinu > 0 && m.row.Value > maxKoinu {
			continue
		}
		filtered = append(filtered, m)
		sum += m.row.Value
	}
	if opts.minimumSumAmount > 0 && sum < int64(opts.minimumSumAmount*1e8) {
		return nil
	}
	if opts.maximumCount > 0 && len(filtered) > opts.maximumCount {
		// Largest-first (Core coin selection hint for fundrawtransaction callers).
		for i := 0; i < len(filtered); i++ {
			for j := i + 1; j < len(filtered); j++ {
				if filtered[j].row.Value > filtered[i].row.Value {
					filtered[i], filtered[j] = filtered[j], filtered[i]
				}
			}
		}
		filtered = filtered[:opts.maximumCount]
	}
	return filtered
}
