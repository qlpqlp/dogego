// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
)

// minSetMaxConnections matches MAX_ADDNODE_CONNECTIONS in Dogecoin Core (src/net.h).
const minSetMaxConnections = 8

// execSetMaxConnections implements setmaxconnections (Core net.cpp) when DataPaths.SetMaxConnections is wired.
func execSetMaxConnections(paths *DataPaths, params []json.RawMessage) (bool, int, string) {
	if len(params) != 1 {
		return false, -8, "setmaxconnections: newconnectioncount required"
	}
	var v float64
	if err := json.Unmarshal(params[0], &v); err != nil {
		return false, -8, "setmaxconnections: bad newconnectioncount"
	}
	if v < float64(minSetMaxConnections) || v > float64(maxSetMaxConnections) || v != float64(int(v)) {
		return false, -8, fmt.Sprintf("setmaxconnections: maxconnectioncount must be an integer between %d and %d", minSetMaxConnections, maxSetMaxConnections)
	}
	if paths == nil || paths.SetMaxConnections == nil {
		return false, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	if err := paths.SetMaxConnections(int(v)); err != nil {
		return false, -8, err.Error()
	}
	return true, 0, ""
}

// execSetNetworkActive implements setnetworkactive (Core net.cpp) when DataPaths.SetNetworkActive is wired.
func execSetNetworkActive(paths *DataPaths, params []json.RawMessage) (bool, int, string) {
	if len(params) != 1 {
		return false, -8, "setnetworkactive: state required"
	}
	var state bool
	if err := json.Unmarshal(params[0], &state); err != nil {
		return false, -8, "setnetworkactive: bad state"
	}
	if paths == nil || paths.SetNetworkActive == nil {
		return false, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	active, err := paths.SetNetworkActive(state)
	if err != nil {
		return false, -8, err.Error()
	}
	return active, 0, ""
}
