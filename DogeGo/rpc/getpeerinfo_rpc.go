// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

// execGetPeerInfoRPC implements getpeerinfo (Core net.cpp).
func execGetPeerInfoRPC(paths *DataPaths) ([]map[string]interface{}, int, string) {
	if !p2pWired(paths) {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	return execGetPeerInfo(paths), 0, ""
}

// execGetConnectionCount implements getconnectioncount (Core net.cpp).
func execGetConnectionCount(paths *DataPaths) (int, int, string) {
	if !p2pWired(paths) {
		return 0, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	n := 0
	if paths != nil && paths.NetworkActive != nil && !paths.NetworkActive() {
		return n, 0, ""
	}
	if paths != nil && paths.ConnectionCount != nil {
		n = paths.ConnectionCount()
	} else if paths != nil && paths.P2PStats != nil {
		if snap := paths.P2PStats(); snap != nil {
			if v, ok := snap["connections_outbound"].(int); ok {
				n = v
			}
			if cin, ok := snap["connections_inbound"].(int); ok {
				n += cin
			}
		}
	}
	return n, 0, ""
}
