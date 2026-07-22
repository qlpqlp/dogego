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

// AttemptEvictInboundForNew picks an inbound peer to disconnect when the inbound
// slots are full (Core AttemptToEvictConnection-style: protect addnode + BIP152 HB,
// prefer crowded /16 netgroups, then oldest).
// Returns the victim address, or "" if none can be evicted.
func (pm *PeerMgr) AttemptEvictInboundForNew() string {
	if pm == nil {
		return ""
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.pickInboundEvictionVictimLocked()
}

type inboundEvictCand struct {
	addr  string
	since time.Time
	group string
}

func (pm *PeerMgr) pickInboundEvictionVictimLocked() string {
	var cands []inboundEvictCand
	groupCount := map[string]int{}
	for addr, l := range pm.sessions {
		if l == nil || !l.inbound || l.primary {
			continue
		}
		if pm.peerWhitelistedLocked(addr) {
			continue // protect addnode / preferred peers (flexible host:port match)
		}
		if l.cmpctHBFrom || l.cmpctHBTo {
			continue // protect high-bandwidth compact-block peers
		}
		g := hostPortGroup16(addr)
		cands = append(cands, inboundEvictCand{addr: addr, since: l.since, group: g})
		groupCount[g]++
	}
	if len(cands) == 0 {
		return ""
	}
	maxG := 0
	for _, n := range groupCount {
		if n > maxG {
			maxG = n
		}
	}
	var victim string
	var victimSince time.Time
	for _, c := range cands {
		// When one /16 is over-represented, prefer victims from that group (eclipse resistance).
		if maxG > 1 && groupCount[c.group] < maxG {
			continue
		}
		if victim == "" || c.since.Before(victimSince) {
			victim = c.addr
			victimSince = c.since
		}
	}
	if victim != "" {
		return victim
	}
	// Fallback: oldest unprotected (should be unreachable if cands non-empty).
	for _, c := range cands {
		if victim == "" || c.since.Before(victimSince) {
			victim = c.addr
			victimSince = c.since
		}
	}
	return victim
}

// acceptInboundOrEvict disconnects the oldest unprotected inbound when full so a new peer can connect.
func (pm *PeerMgr) acceptInboundOrEvict(addr string) bool {
	if pm.inboundCount() < pm.p2p.MaxInbound {
		return true
	}
	victim := pm.AttemptEvictInboundForNew()
	if victim == "" {
		return false
	}
	applog.Line("net", "evicting inbound peer "+victim+" for new inbound "+addr)
	_ = pm.DisconnectPeer(victim)
	return pm.inboundCount() < pm.p2p.MaxInbound
}
