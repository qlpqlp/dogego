// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "strings"

// Core client RPC codes (src/rpc/protocol.h).
const (
	ErrP2PDisabled              = "Error: Peer-to-peer functionality missing or disabled"
	CodeRPCP2PDisabled            = -31
	ErrNodeNotConnected           = "Node not found in connected nodes"
	CodeRPCNodeNotConnected       = -29
	ErrNodeNotAdded               = "Error: Node has not been added."
	CodeRPCNodeNotAdded           = -24
	ErrInvalidIPOrSubnet          = "Error: Invalid IP/Subnet"
	CodeRPCInvalidIPOrSubnet      = -30
	ErrIPAlreadyBanned            = "Error: IP/Subnet already banned"
	CodeRPCNodeAlreadyAdded       = -23
	ErrUnbanFailed                = "Error: Unban failed. Requested address/subnet was not previously banned."
	maxSetMaxConnections          = 32
)

// p2pWired reports whether the node session wired P2P byte counters or peer RPC hooks.
func p2pWired(paths *DataPaths) bool {
	if paths == nil {
		return false
	}
	return paths.NetRecv != nil || paths.ConnectionCount != nil || paths.PeerInfo != nil
}

func mapSetBanError(err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	msg := err.Error()
	switch {
	case msg == ErrInvalidIPOrSubnet, msg == ErrUnbanFailed:
		return CodeRPCInvalidIPOrSubnet, msg
	case msg == ErrIPAlreadyBanned:
		return CodeRPCNodeAlreadyAdded, msg
	default:
		return -8, msg
	}
}

func mapDisconnectError(err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	msg := err.Error()
	if msg == ErrNodeNotConnected || strings.Contains(msg, "Node not found in connected nodes") {
		return CodeRPCNodeNotConnected, ErrNodeNotConnected
	}
	return -8, msg
}
