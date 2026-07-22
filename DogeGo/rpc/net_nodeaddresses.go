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

// execGetNodeAddresses implements getnodeaddresses (Core net.cpp).
func execGetNodeAddresses(paths *DataPaths, params []json.RawMessage) ([]interface{}, int, string) {
	if paths == nil || paths.NodeAddresses == nil {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	count := 1
	network := ""
	if len(params) > 2 {
		return nil, -8, "getnodeaddresses: too many arguments"
	}
	if len(params) >= 1 {
		var n float64
		if err := json.Unmarshal(params[0], &n); err != nil {
			return nil, -8, "getnodeaddresses: bad argument"
		}
		if n < 0 {
			return nil, -8, "getnodeaddresses: count must be non-negative"
		}
		count = int(n)
	}
	if len(params) == 2 {
		var net string
		if err := json.Unmarshal(params[1], &net); err != nil {
			return nil, -8, "getnodeaddresses: bad network argument"
		}
		network = strings.ToLower(strings.TrimSpace(net))
		switch network {
		case "", "ipv4", "ipv6", "onion":
		default:
			return nil, -8, "getnodeaddresses: unknown network "+net
		}
	}
	rows := paths.NodeAddresses(count, network)
	out := make([]interface{}, len(rows))
	for i, row := range rows {
		out[i] = row
	}
	return out, 0, ""
}
