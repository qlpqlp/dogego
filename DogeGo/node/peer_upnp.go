// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

// SetMappedExternal records the router-mapped public endpoint (UPnP / NAT-PMP).
func (pm *PeerMgr) SetMappedExternal(host string, port int, method string) {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	pm.mappedExtHost = host
	pm.mappedExtPort = port
	pm.mappedMethod = method
	pm.mu.Unlock()
}

// ClearMappedExternal clears the last UPnP mapping advertisement.
func (pm *PeerMgr) ClearMappedExternal() {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	pm.mappedExtHost = ""
	pm.mappedExtPort = 0
	pm.mappedMethod = ""
	pm.mu.Unlock()
}

// MappedExternal returns the advertised public host:port and method ("upnp-igd2", …), or empty.
func (pm *PeerMgr) MappedExternal() (host string, port int, method string) {
	if pm == nil {
		return "", 0, ""
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.mappedExtHost, pm.mappedExtPort, pm.mappedMethod
}
