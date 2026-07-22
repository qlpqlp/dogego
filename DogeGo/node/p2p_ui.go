// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"
)

// P2PUIExtras carries optional IBD / peer-score fields for the web dashboard.
type P2PUIExtras struct {
	Assist                   *AssistPeerRegistry
	Scorer                   *BlockPeerScorer
	ChainActiveHeight        int64 // UTXO/connect tip; -1 if unknown
	StoredBodiesHeight       int64 // contiguous raw bodies; -1 if unknown
	IBDLanes                 int
	IBDProgress              map[string]interface{} // blocks_per_minute, in_flight, …
	DedicatedHeaderRunning   bool
	DedicatedHeaderAddr      string
}

// syncPeerCounts aggregates live TCP sessions used for IBD (Core getnetworkinfo-style).
type syncPeerCounts struct {
	connectionsTotal    int
	connectionsInbound  int
	connectionsOutbound int // all outbound sync: relay + primary + block-assist + dedicated header
	relayMgrTotal       int
	outboundRelay       int
	primaryRegistered   bool
	assistN             int
	dedicatedHeader     bool
}

func computeSyncPeerCounts(pm *PeerMgr, primaryAddr string, extras *P2PUIExtras) syncPeerCounts {
	var out syncPeerCounts
	if pm != nil {
		out.relayMgrTotal, out.connectionsInbound, out.outboundRelay = pm.ConnectionBreakdown()
	}
	out.primaryRegistered = primaryAddr != "" && !strings.HasPrefix(strings.TrimSpace(primaryAddr), "(")
	if extras != nil {
		if extras.Assist != nil {
			out.assistN = extras.Assist.Count()
		}
		out.dedicatedHeader = extras.DedicatedHeaderRunning
		if extras.DedicatedHeaderAddr != "" {
			out.dedicatedHeader = out.dedicatedHeader || true
		}
	}
	out.connectionsOutbound = out.outboundRelay
	if out.primaryRegistered {
		out.connectionsOutbound++
	}
	out.connectionsOutbound += out.assistN
	if out.dedicatedHeader {
		out.connectionsOutbound++
	}
	// relayMgrTotal already includes the registered primary; add only non-PeerMgr sync links.
	out.connectionsTotal = out.relayMgrTotal + out.assistN
	if out.dedicatedHeader {
		out.connectionsTotal++
	}
	return out
}

// SyncPeersActive reports whether any sync-related P2P link is up (not still probing seeds).
func SyncPeersActive(primaryAddr string, extras *P2PUIExtras) bool {
	if primaryAddr != "" && !strings.HasPrefix(strings.TrimSpace(primaryAddr), "(") {
		return true
	}
	if extras == nil {
		return false
	}
	if extras.DedicatedHeaderRunning || extras.DedicatedHeaderAddr != "" {
		return true
	}
	return extras.Assist != nil && extras.Assist.Count() > 0
}

// PeerDialingIndicator is true while startup peer probe has not yet attached a sync link.
func PeerDialingIndicator(primaryAddr, peerSlot string, extras *P2PUIExtras) bool {
	if SyncPeersActive(primaryAddr, extras) {
		return false
	}
	if peerSlot != "" && strings.HasPrefix(strings.TrimSpace(peerSlot), "(") {
		return true
	}
	return primaryAddr == ""
}

// BuildP2PUISnapshot returns JSON-friendly P2P status for the web dashboard (/api/p2p, /api/summary).
func BuildP2PUISnapshot(settings P2PModeSettings, pm *PeerMgr, primaryAddr, peerSlot string, extras *P2PUIExtras) map[string]any {
	peerDialing := PeerDialingIndicator(primaryAddr, peerSlot, extras)
	counts := computeSyncPeerCounts(pm, primaryAddr, extras)

	out := map[string]any{
		"wired":              true,
		"p2p_connectivity":   settings.Mode,
		"p2p_description":    settings.Description,
		"listen_enabled":     settings.Listen,
		"max_outbound":       settings.MaxOutbound,
		"max_inbound":        settings.MaxInbound,
		"multi_peer_enabled": settings.MaxOutbound > 1 || settings.Listen,
		"primary_peer":       primaryAddr,
		"peer_dialing":       peerDialing,
	}
	if extras != nil && extras.DedicatedHeaderAddr != "" {
		out["dedicated_header_peer"] = extras.DedicatedHeaderAddr
	}
	if extras != nil {
		out["dedicated_header_running"] = extras.DedicatedHeaderRunning
	}

	if pm != nil {
		tried, newAddrs := pm.AddrBookStats()
		out["addrbook_tried"] = tried
		out["addrbook_new"] = newAddrs
		out["addrbook_tried_max"] = maxAddrBookTried
		out["addrbook_new_max"] = maxAddrBookNew
		tbUsed, nbUsed, tbMax, nbMax := pm.AddrBookBucketStats()
		out["addrbook_tried_buckets_used"] = tbUsed
		out["addrbook_new_buckets_used"] = nbUsed
		out["addrbook_tried_bucket_max_fill"] = tbMax
		out["addrbook_new_bucket_max_fill"] = nbMax
		out["addrbook_tried_buckets_total"] = addrTriedBucketCount
		out["addrbook_new_buckets_total"] = addrNewBucketCount
		out["addrbook_bucket_slot_cap"] = addrBucketSlotCap
		out["addrbook_n_key_set"] = pm.HasAddrmanKey()
		if info := pm.AddrManInfo(); info != nil {
			out["addrman_info"] = info
		}
		out["median_timeoffset"] = pm.MedianTimeOffset()
		if rows := pm.LocalAddressRows(); len(rows) > 0 {
			out["localaddresses"] = rows
		}
		if h, port, method := pm.MappedExternal(); h != "" && port > 0 {
			out["upnp_mapped"] = true
			out["upnp_external"] = h + ":" + itoa(port)
			out["upnp_method"] = method
		}
	}

	out["connections_total"] = counts.connectionsTotal
	out["connections_inbound"] = counts.connectionsInbound
	out["connections_outbound"] = counts.connectionsOutbound
	out["connections_outbound_relay"] = counts.outboundRelay
	out["relay_peers"] = counts.outboundRelay + counts.connectionsInbound
	out["block_assist_connections"] = counts.assistN
	if counts.dedicatedHeader {
		out["dedicated_header_connections"] = 1
	} else {
		out["dedicated_header_connections"] = 0
	}

	if extras != nil {
		if extras.IBDLanes > 0 {
			out["ibd_sync_lanes"] = extras.IBDLanes
		}
		if extras.ChainActiveHeight >= 0 {
			out["chain_active_height"] = extras.ChainActiveHeight
		}
		if extras.StoredBodiesHeight >= 0 {
			out["contiguous_block_height"] = extras.StoredBodiesHeight
		}
		if extras.Assist != nil {
			snaps := extras.Assist.Snapshot()
			if len(snaps) > 0 {
				rows := make([]map[string]any, 0, len(snaps))
				for _, s := range snaps {
					rows = append(rows, map[string]any{
						"addr":       s.Addr,
						"lane":       s.Lane,
						"bytes_recv": s.BytesRecv,
						"bytes_sent": s.BytesSent,
					})
				}
				out["block_assist_peers"] = rows
			}
		}
		if extras.Scorer != nil {
			tops := extras.Scorer.TopPeers(8)
			rows := make([]map[string]any, 0, len(tops))
			for _, e := range tops {
				rows = append(rows, map[string]any{
					"addr":   e.Addr,
					"score":  e.Score,
					"blocks": e.Blocks,
				})
			}
			out["top_block_peers"] = rows
		}
		if len(extras.IBDProgress) > 0 {
			out["ibd_progress"] = extras.IBDProgress
		}
	}

	status, msg := p2pHealthStatus(settings, counts, primaryAddr, peerDialing, extras)
	out["health"] = status
	out["health_message"] = msg
	if hint := p2pInboundHint(settings, counts); hint != "" {
		out["inbound_hint"] = hint
	}
	out["cgnat_mode"] = settings.Mode == P2PModeCGNAT
	out["starlink_hint"] = settings.Mode == P2PModeCGNAT || settings.Mode == P2PModeBoth
	out["networks"] = BuildNetworksInfo(settings)
	annotateCmpctHBCounts(out, pm, false, false)
	return out
}

func p2pHealthStatus(settings P2PModeSettings, counts syncPeerCounts, primary string, dialing bool, extras *P2PUIExtras) (status, message string) {
	syncOut := counts.connectionsOutbound
	total := counts.connectionsTotal
	if dialing && !SyncPeersActive(primary, extras) && syncOut == 0 {
		return "starting", "Connecting to the network…"
	}
	if syncOut > 0 && primary == "" {
		// Split IBD: block-assist and/or dedicated header without primary relay yet.
		msg := "IBD sync active with " + itoa(syncOut) + " outbound link(s)"
		if counts.assistN > 0 {
			msg += " (" + itoa(counts.assistN) + " block-assist"
			if counts.dedicatedHeader {
				msg += ", dedicated headers"
			}
			msg += ")"
		} else if counts.dedicatedHeader {
			msg += " (dedicated headers)"
		}
		msg += "."
		if settings.MaxOutbound > 1 && counts.outboundRelay == 0 {
			return "warming", msg + " Adding relay peers…"
		}
		return "ok", msg
	}
	if dialing || primary == "" || strings.HasPrefix(strings.TrimSpace(primary), "(") {
		if syncOut > 0 {
			return "warming", "Sync peers connected; primary relay still negotiating…"
		}
		return "starting", "Connecting to the network…"
	}
	if settings.Mode == P2PModeCGNAT {
		if total >= 2 || syncOut >= 2 {
			return "ok", "CGNAT mode: outbound relay active (" + itoa(syncOut) + " outbound sync connections)."
		}
		if settings.MaxOutbound > 1 {
			return "warming", "CGNAT mode: primary peer up; dialing additional outbound relay peers…"
		}
		return "degraded", "CGNAT mode: only the primary peer is connected. Set maxoutbound ≥ 2 or wait for more dials."
	}
	if settings.Listen && settings.MaxInbound > 0 && counts.connectionsInbound == 0 && settings.Mode != P2PModeCGNAT {
		if settings.Mode == P2PModeClassic && total < 2 {
			return "warming", "Classic mode: listening for inbound peers; maintain outbound connections."
		}
	}
	if settings.MaxOutbound > 1 && syncOut < 2 && counts.outboundRelay < 1 {
		return "warming", "Multi-peer mode: sync active; adding relay peers…"
	}
	if syncOut >= 2 || total >= 2 {
		return "ok", "P2P active with " + itoa(syncOut) + " outbound sync connection(s)."
	}
	return "single", "Single peer mode (set p2p_connectivity to cgnat or both for multi-peer relay)."
}

// p2pInboundHint explains zero inbound connections when listen mode is enabled but NAT blocks dial-in.
func p2pInboundHint(settings P2PModeSettings, counts syncPeerCounts) string {
	if settings.Mode == P2PModeCGNAT || !settings.Listen || settings.MaxInbound <= 0 {
		return ""
	}
	if counts.connectionsInbound > 0 {
		return ""
	}
	if counts.connectionsOutbound < 1 {
		return ""
	}
	switch settings.Mode {
	case P2PModeBoth:
		return "No inbound peers (normal behind CGNAT/Starlink). Outbound sync is working - use cgnat mode in Settings if you cannot port-forward."
	case P2PModeClassic:
		return "Listening but no inbound peers yet. Forward TCP P2P on your router or switch to cgnat if you are behind carrier-grade NAT."
	default:
		return ""
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// P2PExtrasFromNode builds dashboard extras from live node state.
func P2PExtrasFromNode(assist *AssistPeerRegistry, scorer *BlockPeerScorer, chainActive, storedBodies int64, ibdLanes int, dedicatedRunning bool, dedicatedAddr string) *P2PUIExtras {
	return &P2PUIExtras{
		Assist:                 assist,
		Scorer:                 scorer,
		ChainActiveHeight:      chainActive,
		StoredBodiesHeight:     storedBodies,
		IBDLanes:               ibdLanes,
		DedicatedHeaderRunning: dedicatedRunning,
		DedicatedHeaderAddr:    dedicatedAddr,
	}
}
