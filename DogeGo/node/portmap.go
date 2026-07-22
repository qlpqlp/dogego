// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"time"

	"dogego/applog"
	"dogego/netfw/upnp"
)

const portMapRefresh = 20 * time.Minute

// StartPortMapping maps the P2P port through UPnP/NAT-PMP when enabled (Core -upnp).
// Call the returned stop func on shutdown (also unmaps the router).
func StartPortMapping(ctx context.Context, mode string, listen bool, port int, pm *PeerMgr) (stop func()) {
	if pm == nil || !upnp.ShouldMap(upnp.ParseMode(mode), listen) {
		return func() {}
	}
	runCtx, cancel := context.WithCancel(ctx)
	go portMapLoop(runCtx, port, pm)
	return func() {
		cancel()
		upnp.Unmap()
	}
}

func portMapLoop(ctx context.Context, port int, pm *PeerMgr) {
	tryOnce := func() {
		res := upnp.Map(ctx, port)
		if res.OK && res.ExternalIP != nil {
			pm.SetMappedExternal(res.ExternalIP.String(), res.Port, res.Method)
			applog.Line("net", fmt.Sprintf("UPnP/NAT-PMP: mapped TCP %d → %s:%d (%s)", port, res.ExternalIP, res.Port, res.Method))
			return
		}
		pm.ClearMappedExternal()
		if res.Err != nil {
			applog.Line("net", "UPnP/NAT-PMP: "+res.Err.Error())
		}
	}
	tryOnce()
	ticker := time.NewTicker(portMapRefresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tryOnce()
		}
	}
}
