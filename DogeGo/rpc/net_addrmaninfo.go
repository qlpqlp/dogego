// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "encoding/json"

// execGetAddrmanInfo implements getaddrmaninfo (Core addrman summary; DogeGo subset).
func execGetAddrmanInfo(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if paths == nil || paths.AddrManInfo == nil {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	if len(params) > 0 {
		return nil, -8, "getaddrmaninfo: no arguments expected"
	}
	return paths.AddrManInfo(), 0, ""
}
