// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

// execPing implements ping (Core net.cpp): queue outbound P2P pings; results in getpeerinfo.
func execPing(paths *DataPaths) (interface{}, int, string) {
	if paths == nil || paths.PingPeers == nil {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	paths.PingPeers()
	return nil, 0, ""
}
