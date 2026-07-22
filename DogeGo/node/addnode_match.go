// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"strings"
)

// addnodeMatchesSession reports whether sessionAddr is the same peer as the addnode target.
func addnodeMatchesSession(added, sessionAddr string) bool {
	if added == "" || sessionAddr == "" {
		return false
	}
	if added == sessionAddr {
		return true
	}
	ah, ap, err1 := net.SplitHostPort(added)
	sh, sp, err2 := net.SplitHostPort(sessionAddr)
	if err1 != nil || err2 != nil {
		return false
	}
	if ap != sp {
		return false
	}
	aip := net.ParseIP(ah)
	sip := net.ParseIP(sh)
	if aip != nil && sip != nil {
		return aip.Equal(sip)
	}
	return strings.EqualFold(ah, sh)
}

// ConnectedAddressesForAddedNode returns Core-shaped address rows when a session matches added.
func (pm *PeerMgr) ConnectedAddressesForAddedNode(added string) []interface{} {
	if pm == nil || added == "" {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for addr, link := range pm.sessions {
		if addnodeMatchesSession(added, addr) {
			conn := "outbound"
			if link != nil && link.inbound {
				conn = "inbound"
			}
			return []interface{}{
				map[string]interface{}{
					"address":   addr,
					"connected": conn,
				},
			}
		}
	}
	return nil
}
