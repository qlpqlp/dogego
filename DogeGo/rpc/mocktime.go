// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"

	"dogego/clock"
)

// execSetMockTime sets the local clock used for header validation (Core -regtest mocktime).
func execSetMockTime(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var ts json.Number
	if err := json.Unmarshal(params[0], &ts); err != nil {
		return nil, -8, "setmocktime: timestamp must be a JSON number"
	}
	n, err := ts.Int64()
	if err != nil {
		return nil, -8, "setmocktime: timestamp must be an integer"
	}
	clock.SetMockUnix(n)
	return nil, 0, ""
}

// execGetMockTime returns the active mock timestamp (0 = real time).
func execGetMockTime(params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	return clock.MockUnix(), 0, ""
}
