// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "strings"

// RPCStatusLabel returns dashboard copy for JSON-RPC listener state.
func RPCStatusLabel(rpcAddr string, listening, dispatchReady bool) string {
	if strings.TrimSpace(rpcAddr) == "" {
		return "off"
	}
	if !listening {
		return "starting"
	}
	if !dispatchReady {
		return "warming_up"
	}
	return "ready"
}

// RPCStatusDisplay is human-readable RPC status for the overview strip.
func RPCStatusDisplay(rpcAddr string, listening, dispatchReady bool) string {
	addr := strings.TrimSpace(rpcAddr)
	if addr == "" {
		return "off"
	}
	if dispatchReady {
		return addr
	}
	if listening {
		return addr + " (warming up)"
	}
	return addr + " (starting)"
}

// EnrichRPCSummaryFields adds rpc_enabled, rpc_listening, rpc_dispatch_ready, and rpc_status to summary/capabilities maps.
func EnrichRPCSummaryFields(out map[string]any, rpcAddr string, snapshot func() (listening, dispatchReady bool)) {
	if out == nil {
		return
	}
	out["rpc_enabled"] = strings.TrimSpace(rpcAddr) != ""
	out["rpc_addr"] = rpcAddr
	listening, ready := false, false
	if snapshot != nil {
		listening, ready = snapshot()
	}
	out["rpc_listening"] = listening
	out["rpc_dispatch_ready"] = ready
	out["rpc_status"] = RPCStatusLabel(rpcAddr, listening, ready)
	out["rpc_status_display"] = RPCStatusDisplay(rpcAddr, listening, ready)
}
