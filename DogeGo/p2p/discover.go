// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package p2p provides peer discovery (DNS seeds and fixed seeds). The node uses one
// primary outbound session plus an optional block-assist peer for parallel raw block download.
package p2p

import (
	"context"
	"net"
	"strconv"
	"time"

	"dogego/chain"
)

const dnsSeedLookupTimeout = 12 * time.Second

// DiscoverAddresses returns unique host:port candidates from DNS seeds (A/AAAA via LookupIPAddr)
// then fixed peers from chain params (Core order: DNS seeds, then pnSeed6_*). Results are shuffled
// when more than one. If log is non-nil, it receives short status lines (DNS errors, counts).
func DiscoverAddresses(ctx context.Context, p chain.Params, log func(string)) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(hostport string) {
		if hostport == "" {
			return
		}
		if _, ok := seen[hostport]; ok {
			return
		}
		seen[hostport] = struct{}{}
		out = append(out, hostport)
	}
	portStr := strconv.Itoa(p.Port)
	dnsFound := 0
	for _, h := range p.DNSSeeds {
		lookupCtx, cancel := context.WithTimeout(ctx, dnsSeedLookupTimeout)
		records, err := net.DefaultResolver.LookupIPAddr(lookupCtx, h)
		cancel()
		if err != nil {
			if log != nil {
				log("DNS seed " + h + ": " + err.Error())
			}
			continue
		}
		n := 0
		for _, ipa := range records {
			if ipa.IP == nil {
				continue
			}
			add(net.JoinHostPort(ipa.IP.String(), portStr))
			n++
		}
		dnsFound += n
		if log != nil {
			if n == 0 {
				log("DNS seed " + h + ": no A/AAAA records")
			} else {
				log("DNS seed " + h + ": " + strconv.Itoa(n) + " address(es)")
			}
		}
	}
	for _, fp := range p.FixedPeers {
		add(fp)
	}
	if log != nil {
		msg := "peer discovery: " + strconv.Itoa(dnsFound) + " from DNS, " + strconv.Itoa(len(p.FixedPeers)) + " fixed seeds (deduped total " + strconv.Itoa(len(out)) + ")"
		if len(p.DNSSeeds) > 0 && dnsFound == 0 {
			msg += " - DNS seeds unreachable (Core uses seed.multidoge.org); fixed seeds from chainparamsseeds.h are sufficient"
		}
		log(msg)
	}
	if len(out) > 1 {
		out = PreferIPv4FirstShuffle(out)
	}
	return out
}
