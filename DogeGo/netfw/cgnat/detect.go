// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package cgnat

import (
	"context"
	"net"
	"strings"
	"time"

	"dogego/netfw/upnp"
)

// Likely reports whether this machine is probably behind carrier-grade NAT (CGNAT / Starlink / mobile).
// Used by the setup wizard to auto-configure DogeGo relay CGNAT roles.
func Likely(ctx context.Context, p2pMode, upnpMode string, p2pPort int) bool {
	mode := strings.ToLower(strings.TrimSpace(p2pMode))
	if mode == "cgnat" {
		return true
	}
	if mode != "both" && mode != "classic" {
		return false
	}
	up := strings.ToLower(strings.TrimSpace(upnpMode))
	if up == "disable" || up == "disabled" || up == "0" || up == "false" {
		return true
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	res := upnp.Map(probeCtx, p2pPort)
	defer upnp.Unmap()
	if !res.OK {
		return true
	}
	return externalIPLikelyCGNAT(res.ExternalIP)
}

func externalIPLikelyCGNAT(ip net.IP) bool {
	if ip == nil || len(ip) == 0 || ip.IsLoopback() {
		return true
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}
	if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true // RFC6598 shared address space (common CGNAT)
	}
	return ip4.IsPrivate() || ip4.IsLinkLocalUnicast()
}
