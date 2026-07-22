// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"strings"
)

// P2P connectivity modes (CGNAT / Starlink-friendly full-node networking).
const (
	P2PModeClassic = "classic" // inbound listener + outbound dials (datacenter / port-forwarded home)
	P2PModeCGNAT   = "cgnat"   // outbound-only multi-peer; no public listen (carrier NAT / Starlink)
	P2PModeBoth    = "both"    // inbound listener + aggressive outbound relay (recommended default)
)

// Default P2P peer limits (Core uses 8 outbound within 125 maxconnections; DogeGo defaults higher for IBD).
const (
	defaultMaxOutbound = 12
	defaultMaxInbound  = 16
)

// P2PModeSettings holds effective listen/dial policy after parsing user config.
type P2PModeSettings struct {
	Mode         string
	Listen       bool
	MaxOutbound  int // includes the primary sync peer
	MaxInbound   int
	Description  string
}

// ParseP2PMode normalizes p2p_connectivity and applies limits.
func ParseP2PMode(mode string, maxOut, maxIn int) (P2PModeSettings, error) {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		m = P2PModeBoth
	}
	switch m {
	case P2PModeClassic, P2PModeCGNAT, P2PModeBoth:
	default:
		return P2PModeSettings{}, fmt.Errorf("p2p_connectivity must be classic, cgnat, or both (got %q)", mode)
	}
	out := maxOut
	if out <= 0 {
		out = defaultMaxOutbound
	}
	if out > 32 {
		out = 32
	}
	in := maxIn
	if in < 0 {
		in = 0
	}
	if in > 64 {
		in = 64
	}
	s := P2PModeSettings{Mode: m, MaxOutbound: out}
	switch m {
	case P2PModeClassic:
		s.Listen = true
		if in == 0 {
			in = defaultMaxInbound
		}
		s.MaxInbound = in
		s.Description = "classic: accept inbound on P2P port and maintain outbound peers"
	case P2PModeCGNAT:
		s.Listen = false
		s.MaxInbound = 0
		s.Description = "cgnat: outbound-only multi-peer relay (no inbound listen; works behind carrier NAT)"
	case P2PModeBoth:
		s.Listen = true
		if in == 0 {
			in = defaultMaxInbound
		}
		s.MaxInbound = in
		s.Description = "both: inbound listen plus outbound multi-peer relay (best effort on all links)"
	}
	return s, nil
}
