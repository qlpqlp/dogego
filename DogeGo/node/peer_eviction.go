// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"time"

	"dogego/applog"
)

const (
	relayPeerMaxIdle          = 25 * time.Minute
	relayPeerMinAgeBeforeEvict = 5 * time.Minute
)

// EvictStaleRelayPeers disconnects outbound relay links with no recent inbound traffic (Core-style slot recycling).
func (pm *PeerMgr) EvictStaleRelayPeers() int {
	if pm == nil {
		return 0
	}
	now := time.Now()
	var victims []string
	pm.mu.Lock()
	for addr, l := range pm.sessions {
		if l == nil || l.primary || l.inbound {
			continue
		}
		if now.Sub(l.since) < relayPeerMinAgeBeforeEvict {
			continue
		}
		last := l.lastRecv
		if last.IsZero() {
			last = l.since
		}
		if now.Sub(last) > relayPeerMaxIdle {
			victims = append(victims, addr)
		}
	}
	pm.mu.Unlock()
	for _, addr := range victims {
		applog.Line("net", "evicting idle relay peer "+addr)
		_ = pm.DisconnectPeer(addr)
	}
	return len(victims)
}
