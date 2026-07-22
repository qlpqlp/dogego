// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "time"

// execGetNetTotals implements getnettotals (Core net.cpp).
func execGetNetTotals(paths *DataPaths) (map[string]interface{}, int, string) {
	if paths == nil || paths.NetRecv == nil || paths.NetSent == nil {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	recv := paths.NetRecv()
	sent := paths.NetSent()
	note := "aggregate raw TCP bytes on connected P2P sessions and block-assist IBD workers (message framing included)"
	if paths.P2PStats != nil {
		if snap := paths.P2PStats(); snap != nil {
			if v, ok := snap["multi_peer_enabled"].(bool); ok && v {
				note = "connected P2P peers (primary + relays + inbound) plus block-assist IBD download sessions"
			} else {
				note = "primary P2P session plus block-assist IBD download sessions when active"
			}
		}
	}
	return map[string]interface{}{
		"totalbytesrecv": recv,
		"totalbytessent": sent,
		"timemillis":     time.Now().UnixMilli(),
		"dogego_note":    note,
		"uploadtarget": map[string]interface{}{
			"timeframe":               0,
			"target":                  0,
			"target_reached":          false,
			"serve_historical_blocks": true,
			"bytes_left_in_cycle":     0,
			"time_left_in_cycle":      0,
		},
	}, 0, ""
}
