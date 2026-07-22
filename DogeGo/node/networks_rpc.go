// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

// BuildNetworksInfo returns Core-shaped getnetworkinfo.networks rows (no Tor proxy in DogeGo).
func BuildNetworksInfo(settings P2PModeSettings) []map[string]interface{} {
	ipv4Reachable := true
	ipv4Limited := !settings.Listen // Core: limited when not accepting inbound (no listen bind).
	return []map[string]interface{}{
		{
			"name":                        "ipv4",
			"limited":                     ipv4Limited,
			"reachable":                   ipv4Reachable,
			"proxy":                       "",
			"proxy_randomize_credentials": false,
		},
		{
			"name":                        "ipv6",
			"limited":                     false,
			"reachable":                   true,
			"proxy":                       "",
			"proxy_randomize_credentials": false,
		},
		{
			"name":                        "onion",
			"limited":                     true,
			"reachable":                   false,
			"proxy":                       "",
			"proxy_randomize_credentials": false,
		},
	}
}
