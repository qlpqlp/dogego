// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"dogego/chain"
)

// P2PRelayPeer is a TCP P2P peer advertising NODE_DOGEGO_RELAY_CGNAT.
type P2PRelayPeer struct {
	TCPAddr  string
	Services uint64
}

// DiscoverTargets merges static seeds, persisted learned relays, DNS TXT, and live P2P
// peers advertising NODE_DOGEGO_RELAY_CGNAT into QUIC host:port targets.
// Unknown service bits are ignored by Core/old nodes; only DogeGo clients use this list.
func DiscoverTargets(ctx context.Context, dnsSeed string, staticSeeds, learnedSeeds []string, relayPort int, p2pPeers []P2PRelayPeer) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(hostport string) {
		hostport = normalizeHostPort(hostport, relayPort)
		if hostport == "" {
			return
		}
		if _, ok := seen[hostport]; ok {
			return
		}
		seen[hostport] = struct{}{}
		out = append(out, hostport)
	}
	for _, s := range staticSeeds {
		add(s)
	}
	for _, s := range learnedSeeds {
		add(s)
	}
	for _, host := range splitDNSSeedHosts(dnsSeed) {
		for _, target := range lookupTXTSeeds(ctx, host) {
			add(target)
		}
	}
	for _, p := range p2pPeers {
		if !chain.HasDogeGoRelayCGNAT(p.Services) {
			continue
		}
		host, _, err := net.SplitHostPort(p.TCPAddr)
		if err != nil {
			host = strings.TrimSpace(p.TCPAddr)
		}
		if host == "" {
			continue
		}
		add(net.JoinHostPort(host, strconv.Itoa(relayPort)))
	}
	return out
}

// splitDNSSeedHosts splits relay_dnsseed config (newline or comma separated hostnames).
func splitDNSSeedHosts(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		for _, part := range strings.Split(line, ",") {
			h := strings.TrimSpace(part)
			if h == "" {
				continue
			}
			if _, ok := seen[h]; ok {
				continue
			}
			seen[h] = struct{}{}
			out = append(out, h)
		}
	}
	return out
}

func lookupTXTSeeds(ctx context.Context, host string) []string {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil
	}
	r := net.Resolver{}
	txtCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	records, err := r.LookupTXT(txtCtx, host)
	if err != nil {
		return nil
	}
	var out []string
	for _, rec := range records {
		for _, line := range strings.Fields(rec) {
			line = strings.TrimSpace(line)
			if line != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

func normalizeHostPort(raw string, defaultPort int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, ":") {
		if _, _, err := net.SplitHostPort(raw); err == nil {
			return raw
		}
	}
	if defaultPort <= 0 {
		defaultPort = 24433
	}
	return net.JoinHostPort(raw, strconv.Itoa(defaultPort))
}
