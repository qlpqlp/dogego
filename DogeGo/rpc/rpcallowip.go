// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// RPCAllowList restricts JSON-RPC clients by source IP (Core -rpcallowip).
// Loopback is always permitted. When no subnets are configured, only loopback may connect.
type RPCAllowList struct {
	nets []*net.IPNet
	ips  []net.IP
}

// ParseRPCAllowList parses Core-style rpcallowip entries (IP, CIDR, or IP/mask).
func ParseRPCAllowList(specs []string) (*RPCAllowList, error) {
	a := &RPCAllowList{}
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if strings.Contains(spec, "/") {
			_, ipNet, err := net.ParseCIDR(spec)
			if err != nil {
				return nil, fmt.Errorf("invalid rpcallowip %q: %w", spec, err)
			}
			a.nets = append(a.nets, ipNet)
			continue
		}
		ip := net.ParseIP(spec)
		if ip == nil {
			return nil, fmt.Errorf("invalid rpcallowip %q", spec)
		}
		a.ips = append(a.ips, ip)
	}
	return a, nil
}

// Permits reports whether ip may use JSON-RPC (loopback always allowed).
func (a *RPCAllowList) Permits(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	if a == nil {
		return false
	}
	for _, n := range a.ips {
		if ip.Equal(n) {
			return true
		}
	}
	for _, n := range a.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) net.IP {
	if r == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}

func wrapRPCAllowIP(allow *RPCAllowList, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allow.Permits(clientIP(r)) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
