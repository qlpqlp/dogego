// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"dogego/chain"
	"dogego/pow"
)

// resolveBlockLocation resolves height or display hash using the header journal.
func resolveBlockLocation(j HeaderJournal, params []json.RawMessage) ([32]byte, int64, error) {
	var zero [32]byte
	if len(params) < 1 {
		return zero, 0, fmt.Errorf("block hash or height required")
	}
	var p0 interface{}
	if err := json.Unmarshal(params[0], &p0); err != nil {
		return zero, 0, fmt.Errorf("bad param")
	}
	switch v := p0.(type) {
	case float64:
		if v < 0 || v > float64(math.MaxInt64) || v != float64(int64(v)) {
			return zero, 0, fmt.Errorf("invalid height")
		}
		h := int64(v)
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return zero, 0, fmt.Errorf("height %d not found", h)
		}
		return pow.BlockHashLE(h80), h, nil
	case string:
		s := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(v), "0x"))
		if len(s) != 64 {
			return zero, 0, fmt.Errorf("block hash must be 64 hex characters")
		}
		hashLE, err := chain.Hash256FromDisplayHex(s)
		if err != nil {
			return zero, 0, fmt.Errorf("invalid block hash")
		}
		hi, err := j.HeightByDisplayHash(s)
		if err != nil {
			return zero, 0, fmt.Errorf("block not in header chain")
		}
		return hashLE, hi, nil
	default:
		return zero, 0, fmt.Errorf("unsupported param type")
	}
}
