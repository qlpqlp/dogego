// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

// execUptime returns seconds since paths.Uptime was first observed (node process start).
func execUptime(paths *DataPaths) (interface{}, int, string) {
	if paths == nil || paths.Uptime == nil {
		return int64(0), 0, ""
	}
	return paths.Uptime(), 0, ""
}

// execGetRPCInfo returns a Core-shaped subset describing this JSON-RPC server.
func execGetRPCInfo(paths *DataPaths) map[string]interface{} {
	methods := SupportedMethods()
	methodMap := make(map[string]interface{}, len(methods))
	for _, m := range methods {
		methodMap[m] = map[string]interface{}{}
	}
	out := map[string]interface{}{
		"active_commands":          []interface{}{},
		"authentication_failures":  RPCAuthFailures(),
		"logpath":                  "",
		"method":                   methodMap,
		"rpchost":                  "dogego",
		"dogego_supported_methods": methods,
		"dogego_supported_method_n": len(methods),
	}
	if paths != nil {
		if paths.Uptime != nil {
			out["dogego_uptime_seconds"] = paths.Uptime()
		}
		if paths.ChainDataDir != "" {
			out["dogego_chain_datadir"] = paths.ChainDataDir
		}
		if paths.RPCTLSEnabled {
			out["dogego_rpc_tls"] = true
		}
		if paths.ZmqNotifications != nil {
			if rows := paths.ZmqNotifications(); len(rows) > 0 {
				out["dogego_zmq_notifications"] = rows
			}
		}
	}
	out["dogego_note"] = "active_commands always empty; method map lists supported RPC names (no per-method usage counts)"
	return out
}
