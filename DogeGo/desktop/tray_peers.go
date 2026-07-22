// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"strings"

	"dogego/config"
)

// TrayPeerLink is a peer dashboard entry for dual-network tray menus.
type TrayPeerLink struct {
	Label string
	URL   string
}

// PeerTrayLinks returns dashboard links for sibling instances (dual mainnet + testnet).
func PeerTrayLinks(dataDir, currentNetwork string) []TrayPeerLink {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil
	}
	inst, err := config.LoadInstances(dataDir)
	if err != nil || len(inst.Instances) < 2 {
		return nil
	}
	cur := strings.ToLower(strings.TrimSpace(currentNetwork))
	var out []TrayPeerLink
	for _, e := range inst.Instances {
		netSlug := strings.ToLower(strings.TrimSpace(e.Network))
		if netSlug == "" || netSlug == cur {
			continue
		}
		label := strings.TrimSpace(e.Label)
		if label == "" {
			label = e.Network
		}
		url := dashboardURLFromInstance(e)
		if url == "" {
			continue
		}
		out = append(out, TrayPeerLink{
			Label: "Open " + label + " Dashboard",
			URL:   url,
		})
	}
	return out
}

func dashboardURLFromWebUI(webui string) string {
	webui = strings.TrimSpace(webui)
	if webui == "" {
		return ""
	}
	f := config.File{WebUI: webui}
	return DashboardURL(f)
}

func dashboardURLFromInstance(e config.InstanceEntry) string {
	if p := strings.TrimSpace(e.ConfPath); p != "" {
		if f, err := config.LoadFile(p); err == nil {
			return DashboardURL(f)
		}
	}
	return dashboardURLFromWebUI(e.WebUI)
}
