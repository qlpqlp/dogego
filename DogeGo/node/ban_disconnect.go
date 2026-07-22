// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "net"

// DisconnectBanned closes active sessions whose remote IP matches an active ban.
func (pm *PeerMgr) DisconnectBanned(isBanned func(net.IP) bool) int {
	if pm == nil || isBanned == nil {
		return 0
	}
	pm.mu.Lock()
	var targets []string
	for addr, link := range pm.sessions {
		if link == nil || link.primary {
			continue
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		ip := net.ParseIP(host)
		if ip != nil && isBanned(ip) {
			targets = append(targets, addr)
		}
	}
	pm.mu.Unlock()
	for _, addr := range targets {
		_ = pm.DisconnectPeer(addr)
	}
	return len(targets)
}
