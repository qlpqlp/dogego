// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"net"
	"sync"

	"dogego/applog"
	"dogego/p2p"
)

// dgrTunnelDialState holds process-wide DGR P2P dial settings (phase 2 polish).
type dgrTunnelDialState struct {
	mu sync.RWMutex

	relay       DGRTunnelRelay
	active      func() bool
	preferFirst bool
	magic       [4]byte
}

var dgrDial dgrTunnelDialState

// ConfigureDGRTunnelDial sets global DGR tunnel dial policy. preferFirst tries QUIC tunnel before TCP (CGNAT client).
func ConfigureDGRTunnelDial(relay DGRTunnelRelay, active func() bool, preferFirst bool, magic [4]byte) {
	dgrDial.mu.Lock()
	defer dgrDial.mu.Unlock()
	dgrDial.relay = relay
	dgrDial.active = active
	dgrDial.preferFirst = preferFirst
	dgrDial.magic = magic
}

// ClearDGRTunnelDial disables DGR-assisted P2P dials.
func ClearDGRTunnelDial() {
	ConfigureDGRTunnelDial(nil, nil, false, [4]byte{})
}

// DialP2POutbound dials a P2P peer. Returns tunneled=true when traffic uses DGR P2P_FRAME.
func DialP2POutbound(ctx context.Context, dialer net.Dialer, addr string) (net.Conn, bool, error) {
	dgrDial.mu.RLock()
	relay := dgrDial.relay
	active := dgrDial.active
	preferFirst := dgrDial.preferFirst
	magic := dgrDial.magic
	dgrDial.mu.RUnlock()

	tryTCP := func() (net.Conn, error) {
		c, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			if p2p.ObserveDialError(addr, err) {
				applog.Line("net", "IPv6 dials disabled (network unreachable); preferring IPv4 peers")
			}
		}
		return c, err
	}
	tryDGR := func() (net.Conn, error) {
		if relay == nil || active == nil || !active() {
			return nil, errDGRTunnelUnavailable
		}
		return NewDGRTunnelConn(addr, magic, relay)
	}

	if preferFirst && active != nil && active() {
		if c, err := tryDGR(); err == nil {
			applog.Line("dgr", "CGNAT-first tunnel dial "+addr)
			return c, true, nil
		}
		c, err := tryTCP()
		if err == nil {
			return c, false, nil
		}
		if c2, err2 := tryDGR(); err2 == nil {
			applog.Line("dgr", "outbound DGR tunnel dial "+addr+" (TCP failed: "+err.Error()+")")
			return c2, true, nil
		}
		return nil, false, err
	}

	c, err := tryTCP()
	if err == nil {
		return c, false, nil
	}
	if c2, err2 := tryDGR(); err2 == nil {
		applog.Line("dgr", "outbound DGR tunnel dial "+addr+" (TCP failed: "+err.Error()+")")
		return c2, true, nil
	}
	return nil, false, err
}

var errDGRTunnelUnavailable = &dgrTunnelUnavailableError{}

type dgrTunnelUnavailableError struct{}

func (e *dgrTunnelUnavailableError) Error() string { return "dgr tunnel not active" }
