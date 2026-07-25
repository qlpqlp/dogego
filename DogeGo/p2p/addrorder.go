// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package p2p

import (
	"math/rand"
	"net"
	"strings"
)

// HostPortIsIPv4 reports whether addr is a dialable IPv4 host:port (not bracketed IPv6).
func HostPortIsIPv4(hostport string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(hostport))
	if err != nil || host == "" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.To4() != nil
}

// PreferIPv4First returns addrs with IPv4 endpoints before IPv6, preserving order within each group.
// Core operators on Windows/containers often see unreachable IPv6 routes; try IPv4 first. When
// ObserveDialError has disabled IPv6 dials, IPv6 endpoints are dropped entirely.
func PreferIPv4First(addrs []string) []string {
	addrs = FilterDialAddrs(addrs)
	if len(addrs) < 2 {
		return addrs
	}
	var v4, other []string
	for _, a := range addrs {
		if HostPortIsIPv4(a) {
			v4 = append(v4, a)
		} else {
			other = append(other, a)
		}
	}
	if len(v4) == 0 || len(other) == 0 {
		return addrs
	}
	out := make([]string, 0, len(addrs))
	out = append(out, v4...)
	out = append(out, other...)
	return out
}

// PreferIPv4FirstShuffle shuffles within IPv4 and IPv6 groups separately, then concatenates (v4 first).
func PreferIPv4FirstShuffle(addrs []string) []string {
	addrs = FilterDialAddrs(addrs)
	if len(addrs) < 2 {
		return addrs
	}
	var v4, other []string
	for _, a := range addrs {
		if HostPortIsIPv4(a) {
			v4 = append(v4, a)
		} else {
			other = append(other, a)
		}
	}
	if len(v4) > 1 {
		rand.Shuffle(len(v4), func(i, j int) { v4[i], v4[j] = v4[j], v4[i] })
	}
	if len(other) > 1 {
		rand.Shuffle(len(other), func(i, j int) { other[i], other[j] = other[j], other[i] })
	}
	if len(v4) == 0 {
		return other
	}
	if len(other) == 0 {
		return v4
	}
	out := make([]string, 0, len(addrs))
	out = append(out, v4...)
	out = append(out, other...)
	return out
}
