// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"net"
	"net/http"
	"strings"
)

// BindsBeyondLoopback reports whether listenAddr accepts non-loopback TCP connections.
func BindsBeyondLoopback(listenAddr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(listenAddr))
	if err != nil {
		return false
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	switch host {
	case "", "0.0.0.0", "::", "::0", "*":
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host != "127.0.0.1" && host != "localhost"
	}
	return !ip.IsLoopback()
}

// AllowsRemoteClients reports whether rpcallowip explicitly permits non-loopback clients.
func (a *RPCAllowList) AllowsRemoteClients() bool {
	if a == nil {
		return false
	}
	return len(a.nets) > 0 || len(a.ips) > 0
}

// ConfigRequiresAuth reports whether JSON-RPC must use HTTP Basic auth for safe operation.
func ConfigRequiresAuth(listenAddr string, allow *RPCAllowList) bool {
	if allow != nil && allow.AllowsRemoteClients() {
		return true
	}
	return BindsBeyondLoopback(listenAddr)
}

func wrapRejectRemoteWithoutAuth(auth *RPCAuth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ip != nil && !ip.IsLoopback() && (auth == nil || !auth.enabled()) {
			writeWWWAuthenticate(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}
