// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "encoding/json"

// execEcho returns JSON-RPC params decoded to a JSON array (Core echo / echojson; same behavior server-side).
func execEcho(params []json.RawMessage) ([]interface{}, int, string) {
	out := make([]interface{}, 0, len(params))
	for _, p := range params {
		if len(p) == 0 {
			out = append(out, nil)
			continue
		}
		var v interface{}
		if err := json.Unmarshal(p, &v); err != nil {
			return nil, -8, "echo: invalid argument"
		}
		out = append(out, v)
	}
	return out, 0, ""
}
