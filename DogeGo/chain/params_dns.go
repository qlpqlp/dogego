// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "strings"

// WithDNSSeeds returns a copy of p with extra DNS seed hostnames appended (deduped, order preserved).
// Hostnames must not include a port; discovery uses Params.Port.
func WithDNSSeeds(p Params, extra []string) Params {
	if len(extra) == 0 {
		return p
	}
	seen := make(map[string]struct{}, len(p.DNSSeeds)+len(extra))
	var merged []string
	add := func(h string) {
		h = strings.TrimSpace(strings.ToLower(h))
		if h == "" {
			return
		}
		if i := strings.IndexByte(h, ':'); i >= 0 {
			h = strings.TrimSpace(h[:i])
		}
		if h == "" {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		merged = append(merged, h)
	}
	for _, h := range p.DNSSeeds {
		add(h)
	}
	for _, h := range extra {
		add(h)
	}
	out := p
	out.DNSSeeds = merged
	return out
}

// WithoutDNSSeeds returns a copy of p with chain DNS seed hostnames cleared (Core -dnsseed=0).
func WithoutDNSSeeds(p Params) Params {
	out := p
	out.DNSSeeds = nil
	return out
}
