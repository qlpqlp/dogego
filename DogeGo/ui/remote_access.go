// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"dogego/config"
	"dogego/ui/websecurity"
)

func webUIBindsBeyondLoopback(listenAddr string) bool {
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

func remoteDashboardAuthRequired(cfg config.File, listenAddr string) bool {
	if !cfg.WebUIRemoteAuth {
		return false
	}
	return webUIBindsBeyondLoopback(listenAddr)
}

// requireDashboardRead allows loopback reads always; when webui_remote_auth is on and the
// dashboard binds beyond loopback, non-loopback clients must present a valid PIN session.
func requireDashboardRead(w http.ResponseWriter, r *http.Request, gate *websecurity.Gate, cfg config.File, listenAddr string) bool {
	if isLoopback(r) {
		return true
	}
	if !remoteDashboardAuthRequired(cfg, listenAddr) {
		return true
	}
	if gate == nil || !gate.Enabled() {
		writeRemoteAuthErr(w, "remote_auth_requires_pin")
		return false
	}
	return gate.RequireUnlocked(w, r)
}

func writeRemoteAuthErr(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":              code,
		"pin_required":       true,
		"remote_auth":        true,
	})
}
