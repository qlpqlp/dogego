// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package p2p

import (
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
)

// ipv6DialsDisabled is set after an outbound IPv6 dial fails with ENETUNREACH / EHOSTUNREACH
// (typical on containers and Windows hosts without a working IPv6 route). Further PreferIPv4First
// / FilterDialAddrs calls drop bracketed IPv6 endpoints so block-assist and relay dials stop
// hammering unreachable AAAA peers.
var ipv6DialsDisabled atomic.Bool

// IPv6DialsDisabled reports whether outbound IPv6 dials were auto-disabled after network unreachable.
func IPv6DialsDisabled() bool {
	return ipv6DialsDisabled.Load()
}

// ResetIPv6DialGateForTest clears the process-wide IPv6 skip flag (tests only).
func ResetIPv6DialGateForTest() {
	ipv6DialsDisabled.Store(false)
}

// HostPortIsIPv6 reports whether addr is a dialable IPv6 host:port (bracketed literal).
func HostPortIsIPv6(hostport string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(hostport))
	if err != nil || host == "" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.To4() == nil
}

// IsNetworkUnreachable reports OS "no route" style dial failures (Linux/Windows wording).
func IsNetworkUnreachable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ENETUNREACH) || errors.Is(err, syscall.EHOSTUNREACH) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno == syscall.ENETUNREACH || errno == syscall.EHOSTUNREACH {
			return true
		}
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "network is unreachable") ||
		strings.Contains(s, "unreachable network") ||
		strings.Contains(s, "no route to host")
}

// ObserveDialError records an outbound TCP dial failure. When addr is IPv6 and the error is
// network-unreachable, outbound IPv6 dials are disabled for the process. Returns true the first
// time the gate flips so callers can log once.
func ObserveDialError(addr string, err error) (disabledNow bool) {
	if err == nil || !HostPortIsIPv6(addr) || !IsNetworkUnreachable(err) {
		return false
	}
	if ipv6DialsDisabled.Swap(true) {
		return false
	}
	return true
}

// FilterDialAddrs drops IPv6 host:ports when IPv6 dials are disabled; otherwise returns addrs.
func FilterDialAddrs(addrs []string) []string {
	if len(addrs) == 0 || !IPv6DialsDisabled() {
		return addrs
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if HostPortIsIPv6(a) {
			continue
		}
		out = append(out, a)
	}
	return out
}
