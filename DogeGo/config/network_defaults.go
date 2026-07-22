// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "strings"

// ApplyRecommendedNetworkDefaults sets P2P/firewall/UPnP wizard defaults for full nodes.
func ApplyRecommendedNetworkDefaults(f *File) {
	if f == nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(f.NodeMode)) == "spv" {
		if strings.TrimSpace(f.P2PConnectivity) == "" {
			f.P2PConnectivity = "cgnat"
		}
		return
	}
	if strings.TrimSpace(f.P2PConnectivity) == "" {
		f.P2PConnectivity = "both"
	}
	if strings.TrimSpace(f.Firewall) == "" {
		f.Firewall = "auto"
	}
	if strings.TrimSpace(f.Upnp) == "" {
		f.Upnp = "auto"
	}
	if !f.DogeGoRelayCGNAT.UserConfigured() {
		mode := strings.ToLower(strings.TrimSpace(f.P2PConnectivity))
		if mode == "both" || mode == "classic" || mode == "cgnat" {
			ApplyWizardDGRDefaults(f, mode == "cgnat")
		}
	}
	ApplyTestnetAutoMine(f)
}
