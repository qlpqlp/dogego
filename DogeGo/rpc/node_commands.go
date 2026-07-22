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

// execAddNode implements addnode (Core net.cpp) when DataPaths.AddNode is wired.
func execAddNode(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 2 {
		return nil, -8, "addnode: node and command required"
	}
	var node, command string
	if err := json.Unmarshal(params[0], &node); err != nil {
		return nil, -8, "addnode: bad node"
	}
	if err := json.Unmarshal(params[1], &command); err != nil {
		return nil, -8, "addnode: bad command"
	}
	node = strings.TrimSpace(node)
	if len(node) > 256 {
		return nil, -8, "addnode: node address is invalid"
	}
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd != "add" && cmd != "remove" && cmd != "onetry" {
		return nil, -8, "addnode: unknown command"
	}
	if paths == nil || paths.AddNode == nil {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	if err := paths.AddNode(node, cmd); err != nil {
		return nil, -8, err.Error()
	}
	return nil, 0, ""
}

// execDisconnectNode implements disconnectnode (Core net.cpp) when DataPaths.DisconnectNode is wired.
func execDisconnectNode(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 {
		return nil, -8, "disconnectnode: address required"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "disconnectnode: bad address"
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, -8, "disconnectnode: address required"
	}
	if paths == nil || paths.DisconnectNode == nil {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	if err := paths.DisconnectNode(addr); err != nil {
		code, msg := mapDisconnectError(err)
		return nil, code, msg
	}
	return nil, 0, ""
}
