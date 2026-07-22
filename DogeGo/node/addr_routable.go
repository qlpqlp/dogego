// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// IsRelayHostPort reports whether host:port is a valid DGR relay QUIC target (IP or DNS name).
func IsRelayHostPort(hostport string) bool {
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(hostport))
	if err != nil || host == "" {
		return false
	}
	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || p == 0 {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return IsIPPortRoutable(ip, uint16(p))
	}
	if len(host) > 253 || strings.ContainsAny(host, " \t\r\n/") {
		return false
	}
	return true
}

// IsHostPortRoutable reports whether host:port is suitable for addrman (Core CNetAddr::IsRoutable + non-zero port).
func IsHostPortRoutable(hostport string) bool {
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(hostport))
	if err != nil || host == "" {
		return false
	}
	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || p == 0 {
		return false
	}
	port := uint16(p)
	return IsIPPortRoutable(net.ParseIP(host), port)
}

// IsIPPortRoutable matches Core addrman acceptance: valid public/unroutable-excluded addresses.
func IsIPPortRoutable(ip net.IP, port uint16) bool {
	if ip == nil || port == 0 {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		return isRoutableIPv4(ip4)
	}
	ip6 := ip.To16()
	if ip6 == nil {
		return false
	}
	return isRoutableIPv6(ip6)
}

func isRoutableIPv4(ip net.IP) bool {
	if len(ip) != 4 {
		return false
	}
	if ip.IsUnspecified() || ip.Equal(net.IPv4zero) {
		return false
	}
	if ip[0] == 0 || ip[0] == 127 {
		return false
	}
	if ip[0] == 10 {
		return false
	}
	if ip[0] == 172 && ip[1]&0xf0 == 16 {
		return false
	}
	if ip[0] == 192 && ip[1] == 168 {
		return false
	}
	if ip[0] == 169 && ip[1] == 254 {
		return false
	}
	if ip[0] >= 224 && ip[0] <= 239 {
		return false
	}
	if ip[0] == 192 && ip[1] == 0 {
		return false
	}
	if ip[0] == 198 && (ip[1] == 18 || ip[1] == 19) {
		return false
	}
	if ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127 {
		return false
	}
	return true
}

func isRoutableIPv6(ip net.IP) bool {
	if len(ip) != 16 || ip.Equal(net.IPv6zero) {
		return false
	}
	if ip[0] == 0xff {
		return false
	}
	if ip[0] == 0xfe && ip[1]&0xc0 == 0x80 {
		return false
	}
	if ip[0] == 0xfc || ip[0] == 0xfd {
		return false
	}
	if ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x0d && ip[3] == 0xb8 {
		return false
	}
	return true
}

// addrGroup16 returns a Core-style source/address group key (/16 for IPv4) for diversity scoring.
func addrGroup16(ip net.IP) string {
	if ip4 := ip.To4(); ip4 != nil {
		return ipv4GroupKey(ip4)
	}
	if ip6 := ip.To16(); ip6 != nil && ip.To4() == nil {
		return ipv6GroupKey(ip6)
	}
	return ""
}

// hostPortGroup16 returns addrGroup16 for host:port, or "_" when the host is not a parsable IP.
func hostPortGroup16(hostport string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(hostport))
	if err != nil || host == "" {
		return "_"
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "_"
	}
	g := addrGroup16(ip)
	if g == "" {
		return "_"
	}
	return g
}

// SpreadHostPortsByGroup16 round-robins host:ports by IPv4/IPv6 /16 so parallel dials (header probe,
// assist refresh) try distinct neighborhoods early - Core addrman diversity analogue.
func SpreadHostPortsByGroup16(addrs []string) []string {
	if len(addrs) < 2 {
		return addrs
	}
	queues := make(map[string][]string)
	var groupOrder []string
	for _, a := range addrs {
		g := hostPortGroup16(a)
		if len(queues[g]) == 0 {
			groupOrder = append(groupOrder, g)
		}
		queues[g] = append(queues[g], a)
	}
	out := make([]string, 0, len(addrs))
	for len(out) < len(addrs) {
		progress := false
		for _, g := range groupOrder {
			q := queues[g]
			if len(q) == 0 {
				continue
			}
			out = append(out, q[0])
			queues[g] = q[1:]
			progress = true
		}
		if !progress {
			break
		}
	}
	return out
}

func ipv4GroupKey(ip4 net.IP) string {
	return fmt.Sprintf("%d.%d/16", ip4[0], ip4[1])
}

func ipv6GroupKey(ip6 net.IP) string {
	return fmt.Sprintf("%02x%02x/16", ip6[0], ip6[1])
}

// normalizeAddrSeenUnix clamps gossip nTime to Core addrman rules (no future >10m; zero → now).
func normalizeAddrSeenUnix(seenUnix, nowUnix int64) int64 {
	if nowUnix <= 0 {
		nowUnix = 0
	}
	if seenUnix <= 0 {
		if nowUnix > 0 {
			return nowUnix
		}
		return 0
	}
	if nowUnix > 0 && seenUnix > nowUnix+addrMaxFutureOffsetSec {
		return nowUnix
	}
	// Core ignores ancient addr nTime (>30d); treat as freshly learned.
	if nowUnix > 0 && seenUnix < nowUnix-addrStaleAfterSeconds {
		return nowUnix
	}
	return seenUnix
}
