// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "encoding/json"

// execGetZMQNotifications implements Core getzmqnotifications (no params; empty array when ZMQ off).
func execGetZMQNotifications(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	if paths == nil || paths.ZmqNotifications == nil {
		return []interface{}{}, 0, ""
	}
	rows := paths.ZmqNotifications()
	if len(rows) == 0 {
		return []interface{}{}, 0, ""
	}
	out := make([]interface{}, len(rows))
	for i, row := range rows {
		out[i] = row
	}
	return out, 0, ""
}
