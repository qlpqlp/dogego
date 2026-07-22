// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package upnp

import (
	"fmt"
	"net"
)

// localIPv4 returns the preferred outbound LAN address (UDP trick).
func localIPv4() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return nil, fmt.Errorf("upnp: no local UDP address")
	}
	if ip4 := addr.IP.To4(); ip4 != nil {
		return ip4, nil
	}
	return nil, fmt.Errorf("upnp: local address is not IPv4")
}

func guessGateway(ip net.IP) net.IP {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}
	return net.IPv4(ip4[0], ip4[1], ip4[2], 1)
}
