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

// execSetBan implements setban (Core net.cpp) when DataPaths.BanManager is wired.
func execSetBan(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if paths == nil || paths.BanManager == nil {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	if len(params) < 2 {
		return nil, -8, "setban: subnet and command required"
	}
	var subnet, command string
	if err := json.Unmarshal(params[0], &subnet); err != nil {
		return nil, -8, "setban: bad subnet"
	}
	if err := json.Unmarshal(params[1], &command); err != nil {
		return nil, -8, "setban: bad command"
	}
	var banTime int64
	absolute := false
	if len(params) >= 3 && string(params[2]) != "null" {
		var bt float64
		if err := json.Unmarshal(params[2], &bt); err != nil {
			return nil, -8, "setban: bad bantime"
		}
		if bt != float64(int64(bt)) {
			return nil, -8, "setban: bantime must be an integer"
		}
		banTime = int64(bt)
	}
	if len(params) >= 4 && string(params[3]) != "null" {
		if err := json.Unmarshal(params[3], &absolute); err != nil {
			return nil, -8, "setban: bad absolute flag"
		}
	}
	if err := paths.BanManager.SetBan(subnet, command, banTime, absolute); err != nil {
		code, msg := mapSetBanError(err)
		return nil, code, msg
	}
	if strings.EqualFold(strings.TrimSpace(command), "add") && paths.BanDisconnect != nil {
		paths.BanDisconnect()
	}
	return nil, 0, ""
}
