// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// RPCAuth enables HTTP Basic authentication on the JSON-RPC listener when User is non-empty.
// Password may be empty (matches common dogecoin.conf setups).
type RPCAuth struct {
	User     string
	Password string
	Allow    *RPCAllowList // nil = loopback-only (Core default without -rpcallowip)
	Limits   RPCLimits
}

func (a *RPCAuth) enabled() bool {
	return a != nil && strings.TrimSpace(a.User) != ""
}

func rpcAuthOK(a *RPCAuth, r *http.Request) bool {
	u, p, ok := r.BasicAuth()
	if !ok {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(strings.TrimSpace(u)), []byte(strings.TrimSpace(a.User))) == 1
	passOK := subtle.ConstantTimeCompare([]byte(p), []byte(a.Password)) == 1
	return userOK && passOK
}

func writeWWWAuthenticate(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="jsonrpc"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func wrapBasicAuth(auth *RPCAuth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rpcAuthOK(auth, r) {
			writeWWWAuthenticate(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func wrapIfAuth(auth *RPCAuth, next http.Handler) http.Handler {
	var allow *RPCAllowList
	var limits RPCLimits
	if auth != nil {
		allow = auth.Allow
		limits = auth.Limits
	}
	h := wrapRPCLimits(limits, auth, next)
	if auth != nil && auth.enabled() && limits.authFailCap(true) <= 0 {
		h = wrapBasicAuth(auth, h)
	}
	h = wrapRejectRemoteWithoutAuth(auth, h)
	return wrapRPCAllowIP(allow, h)
}
