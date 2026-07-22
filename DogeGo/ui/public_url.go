// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"net"
	"strings"
)

// publicDashboardURL builds the browser URL from the configured listen host and the
// bound port. When webui is "localhost:2013", httptls binds 127.0.0.1/::1, so
// ln.Addr() is an IP; we keep "localhost" in the URL for WebAuthn/biometrics.
func publicDashboardURL(scheme, listenAddr string, ln net.Listener) string {
	if scheme == "" {
		scheme = "http"
	}
	bound := ""
	if ln != nil {
		bound = ln.Addr().String()
	}
	_, boundPort, boundErr := net.SplitHostPort(bound)
	host, port, err := net.SplitHostPort(strings.TrimSpace(listenAddr))
	if err != nil || host == "" {
		if bound != "" {
			return scheme + "://" + bound + "/"
		}
		return scheme + "://localhost:2013/"
	}
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "0.0.0.0", "::", "::0", "*":
		if bound != "" {
			return scheme + "://" + bound + "/"
		}
	}
	if port == "0" && boundErr == nil && boundPort != "" {
		port = boundPort
	}
	return scheme + "://" + net.JoinHostPort(host, port) + "/"
}
