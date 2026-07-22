// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"strings"
)

// RPCWhitelist restricts JSON-RPC to named methods when non-empty (Core-style safe subset).
// ping, help, uptime, and getrpcinfo are always permitted so operators can probe the server.
type RPCWhitelist map[string]struct{}

// ParseRPCWhitelist builds a set from dogecoinconf.json rpcwhitelist entries.
func ParseRPCWhitelist(methods []string) RPCWhitelist {
	if len(methods) == 0 {
		return nil
	}
	w := make(RPCWhitelist, len(methods))
	for _, m := range methods {
		m = strings.ToLower(strings.TrimSpace(m))
		if m != "" {
			w[m] = struct{}{}
		}
	}
	return w
}

// Allowed reports whether method may run under the whitelist (nil whitelist = allow all).
func (w RPCWhitelist) Allowed(method string) bool {
	if w == nil {
		return true
	}
	method = strings.ToLower(strings.TrimSpace(method))
	if method == "" {
		return false
	}
	switch method {
	case "ping", "help", "uptime", "getrpcinfo", "echo", "echojson":
		return true
	}
	_, ok := w[method]
	return ok
}
