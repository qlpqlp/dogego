// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "net"

// LocalAddressRows returns Core-shaped localaddresses entries for getnetworkinfo.
func (pm *PeerMgr) LocalAddressRows() []map[string]interface{} {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	type addrKey struct {
		host string
		port int
	}
	seen := make(map[addrKey]struct{})
	var out []map[string]interface{}
	add := func(host string, port, score int) {
		if host == "" || port <= 0 {
			return
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
			host = "0.0.0.0"
		}
		k := addrKey{host: host, port: port}
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		out = append(out, map[string]interface{}{
			"address": host,
			"port":    port,
			"score":   score,
		})
	}
	if pm.mappedExtHost != "" && pm.mappedExtPort > 0 {
		add(pm.mappedExtHost, pm.mappedExtPort, 8)
	}
	if pm.p2p.Listen && pm.listenPort > 0 {
		host := pm.listenHost
		if host == "" {
			host = "0.0.0.0"
		}
		add(host, pm.listenPort, 4)
	}
	for _, l := range pm.sessions {
		if l == nil || l.conn == nil {
			continue
		}
		tcp, ok := l.conn.LocalAddr().(*net.TCPAddr)
		if !ok {
			continue
		}
		add(tcp.IP.String(), tcp.Port, 1)
	}
	return out
}
