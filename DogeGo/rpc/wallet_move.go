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

// execMoveWallet implements deprecated Core move for the built-in wallet (account bookkeeping only).
func execMoveWallet(params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 3 || len(params) > 5 {
		return nil, -32602, "Wrong number of arguments"
	}
	if _, code, msg := parseRPCAccountLabel(params[0], "move", "fromaccount"); code != 0 {
		return nil, code, msg
	}
	if _, code, msg := parseRPCAccountLabel(params[1], "move", "toaccount"); code != 0 {
		return nil, code, msg
	}
	amt, code, msg := parseMoveAmount(params[2])
	if code != 0 {
		return nil, code, msg
	}
	if amt <= 0 {
		return nil, -3, "Invalid amount for send"
	}
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		var n json.Number
		if err := json.Unmarshal(params[3], &n); err != nil {
			return nil, -8, "move: minconf must be a number"
		}
		if _, err := n.Int64(); err != nil {
			return nil, -8, "move: minconf must be a number"
		}
	}
	if len(params) > 4 && strings.TrimSpace(string(params[4])) != "null" {
		var comment string
		if err := json.Unmarshal(params[4], &comment); err != nil {
			return nil, -8, "move: comment must be a string"
		}
	}
	return true, 0, ""
}
