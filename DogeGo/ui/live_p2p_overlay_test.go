// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"testing"
)

func TestOverlayP2PConnectionCountsUpdatesSummaryWithoutIBDProgress(t *testing.T) {
	sum := map[string]any{
		"connections_out": 0,
		"connections_in":  0,
		"tip_height":      int64(100),
	}
	p2p := map[string]any{
		"connections_outbound": 12,
		"connections_inbound":  3,
		"p2p_connectivity":     "both",
		"health":               "ok",
		"health_message":       "Multi-peer sync active",
		// Intentionally no ibd_progress — overlay must still refresh peer counts.
	}
	overlayP2PProgressOnSummary(sum, p2p)
	if sum["connections_out"] != 12 || sum["connections_in"] != 3 {
		t.Fatalf("connections out=%v in=%v", sum["connections_out"], sum["connections_in"])
	}
	if sum["p2p_connectivity"] != "both" || sum["p2p_health"] != "ok" {
		t.Fatalf("p2p fields: %#v", sum)
	}
	if sum["relay_note"] != "Multi-peer sync active" {
		t.Fatalf("relay_note=%v", sum["relay_note"])
	}
}

func TestOverlayP2PProgressCopiesHeaderAndBodyRates(t *testing.T) {
	sum := map[string]any{"tip_height": int64(100), "chain_active_height": int64(10), "contiguous_raw_height": int64(50)}
	p2p := map[string]any{
		"ibd_progress": map[string]any{
			"blocks_per_minute":            1234.5,
			"contiguous_blocks_per_minute": 800.0,
			"headers_per_minute":           20000.0,
		},
	}
	overlayP2PProgressOnSummary(sum, p2p)
	if sum["blocks_per_minute"] != 1234.5 {
		t.Fatalf("blocks_per_minute=%v", sum["blocks_per_minute"])
	}
	if sum["headers_per_minute"] != 20000.0 {
		t.Fatalf("headers_per_minute=%v", sum["headers_per_minute"])
	}
	if sum["contiguous_blocks_per_minute"] != 800.0 {
		t.Fatalf("contiguous=%v", sum["contiguous_blocks_per_minute"])
	}
}

func TestOverlayP2PProgressClearsDiskSnapshotFlags(t *testing.T) {
	sum := map[string]any{
		"from_disk_snapshot": true,
		"summary_stale":      true,
		"tip_height":         int64(100),
	}
	p2p := map[string]any{
		"contiguous_block_height": 55013,
		"ibd_progress": map[string]any{
			"blocks_per_minute": 12.5,
		},
	}
	overlayP2PProgressOnSummary(sum, p2p)
	if sum["from_disk_snapshot"] != nil || sum["summary_stale"] != nil {
		t.Fatalf("live overlay must clear stale flags: %#v", sum)
	}
	if sum["blocks_per_minute"] != 12.5 {
		t.Fatalf("blocks_per_minute=%v", sum["blocks_per_minute"])
	}
}

func TestSanitizeLiveJSONIfP2PActiveClearsDiskSnapshot(t *testing.T) {
	in, _ := json.Marshal(map[string]any{
		"ok":                 true,
		"from_disk_snapshot": true,
		"summary_stale":      true,
		"summary": map[string]any{
			"from_disk_snapshot": true,
			"summary_stale":      true,
			"tip_height":         float64(100),
			"sync_status_line":   "Showing last known data · refreshing…",
		},
		"p2p": map[string]any{
			"wired":                   true,
			"contiguous_block_height": 55000,
			"ibd_progress": map[string]any{
				"blocks_per_minute": 200.0,
			},
			"dogego_sync_activity": map[string]any{
				"headline": "Downloading block bodies",
			},
		},
	})
	out := sanitizeLiveJSONIfP2PActive(in)
	if len(out) == 0 {
		t.Fatal("expected sanitized live JSON")
	}
	var live map[string]any
	if err := json.Unmarshal(out, &live); err != nil {
		t.Fatal(err)
	}
	if live["from_disk_snapshot"] != nil || live["summary_stale"] != nil {
		t.Fatalf("envelope still stale: %#v", live)
	}
	sum, _ := live["summary"].(map[string]any)
	if sum["from_disk_snapshot"] != nil || sum["summary_stale"] != nil {
		t.Fatalf("summary still stale: %#v", sum)
	}
	if sum["blocks_per_minute"] != 200.0 {
		t.Fatalf("blocks_per_minute=%v", sum["blocks_per_minute"])
	}
}

func TestP2PHasLiveProgressIgnoresZeroedDiskSnapshot(t *testing.T) {
	p2p := map[string]any{
		"from_disk_snapshot":       true,
		"contiguous_block_height":  674269,
		"connections_outbound":     0,
		"block_assist_connections": 17,
		"health_message":           "P2P active with 18 outbound sync connection(s).",
	}
	if p2PHasLiveProgress(p2p) {
		t.Fatal("zeroed disk snapshot must not count as live progress")
	}
	delete(p2p, "from_disk_snapshot")
	if !p2PHasLiveProgress(p2p) {
		t.Fatal("assist connections without from_disk should count as live")
	}
}

func TestOverlayRebuildsOutboundFromAssistWhenZeroed(t *testing.T) {
	sum := map[string]any{"connections_out": 0}
	p2p := map[string]any{
		"connections_outbound":         0,
		"block_assist_connections":     17,
		"dedicated_header_connections": 0,
		"primary_peer":                 "1.2.3.4:22556",
	}
	overlayP2PConnectionCounts(sum, p2p)
	if sum["connections_out"] != 18 {
		t.Fatalf("rebuilt out=%v want 18", sum["connections_out"])
	}
}

func TestOverlayKeepsPeersZeroOnDiskSnapshot(t *testing.T) {
	sum := map[string]any{
		"connections_out":        0,
		"from_disk_snapshot":     true,
		"blocks_per_minute":      999.0,
		"summary_stale":          true,
	}
	p2p := map[string]any{
		"from_disk_snapshot":       true,
		"connections_outbound":     0,
		"block_assist_connections": 17,
		"primary_peer":             "1.2.3.4:22556",
		"ibd_progress": map[string]any{
			"blocks_per_minute": 250.0,
		},
		"health_message": "P2P active with 18 outbound sync connection(s).",
	}
	overlayP2PProgressOnSummary(sum, p2p)
	if sum["connections_out"] != 0 {
		t.Fatalf("disk bootstrap must keep peers at 0, got %v", sum["connections_out"])
	}
	if sum["blocks_per_minute"] != 999.0 {
		// Early return must not overwrite with stale disk ibd_progress either.
		t.Fatalf("disk overlay must not apply rates; blocks_per_minute=%v", sum["blocks_per_minute"])
	}
	if sum["from_disk_snapshot"] != true || sum["summary_stale"] != true {
		t.Fatalf("disk bootstrap must keep stale flags: %#v", sum)
	}
}

func TestEnrichP2PSnapFromRPCMergesGetPeerInfoCounts(t *testing.T) {
	cfg := StartConfig{
		RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
			if method != "getpeerinfo" {
				return nil
			}
			return map[string]interface{}{
				"jsonrpc": "1.0",
				"id":      1,
				"result": []interface{}{
					map[string]interface{}{"addr": "1.2.3.4:22556", "inbound": false},
					map[string]interface{}{"addr": "5.6.7.8:22556", "inbound": false},
					map[string]interface{}{"addr": "9.9.9.9:22556", "inbound": true},
				},
			}
		},
	}
	snap := map[string]any{
		"wired":                true,
		"peer_dialing":         true,
		"connections_outbound": 0,
		"ibd_progress":         map[string]any{"blocks_per_minute": 42.0},
		"dogego_sync_activity": map[string]any{
			"headline": "Connecting to peers",
			"detail":   "Dialing…",
		},
	}
	enriched := enrichP2PSnapFromRPC(cfg, snap)
	if enriched["connections_outbound"] != 2 {
		t.Fatalf("outbound=%v want 2", enriched["connections_outbound"])
	}
	if enriched["connections_inbound"] != 1 {
		t.Fatalf("inbound=%v want 1", enriched["connections_inbound"])
	}
	if enriched["peer_dialing"] != false {
		t.Fatalf("peer_dialing=%v", enriched["peer_dialing"])
	}
	if prog, _ := enriched["ibd_progress"].(map[string]any); prog["blocks_per_minute"] != 42.0 {
		t.Fatalf("ibd_progress must be preserved: %#v", enriched["ibd_progress"])
	}
	act, _ := enriched["dogego_sync_activity"].(map[string]any)
	if act == nil || act["headline"] == "Connecting to peers" {
		t.Fatalf("stale connecting activity must be replaced: %#v", act)
	}
}

func TestEnrichClearsConnectingWhenPeersAlreadyPresent(t *testing.T) {
	cfg := StartConfig{}
	snap := map[string]any{
		"wired":                true,
		"connections_outbound": 12,
		"peer_dialing":         false,
		"dogego_sync_activity": map[string]any{"headline": "Connecting to peers"},
		"ibd_progress":         map[string]any{"blocks_per_minute": 100.0},
	}
	enriched := enrichP2PSnapFromRPC(cfg, snap)
	act, _ := enriched["dogego_sync_activity"].(map[string]any)
	if act == nil || act["headline"] == "Connecting to peers" {
		t.Fatalf("want syncing headline, got %#v", act)
	}
	if enriched["ibd_progress"] == nil {
		t.Fatal("ibd_progress dropped")
	}
}

func TestSanitizeLeavesDiskBootstrapAlone(t *testing.T) {
	in, _ := json.Marshal(map[string]any{
		"ok":                 true,
		"from_disk_snapshot": true,
		"summary_stale":      true,
		"summary": map[string]any{
			"from_disk_snapshot": true,
			"connections_out":    0,
		},
		"p2p": map[string]any{
			"from_disk_snapshot":       true,
			"block_assist_connections": 17,
			"health_message":           "P2P active with 18 outbound sync connection(s).",
		},
	})
	if out := sanitizeLiveJSONIfP2PActive(in); len(out) != 0 {
		t.Fatalf("disk bootstrap must not be promoted to live: %s", out)
	}
}

func TestSanitizePromotesPeerDialingDiskBootstrap(t *testing.T) {
	in, _ := json.Marshal(map[string]any{
		"ok":                 true,
		"from_disk_snapshot": true,
		"summary_stale":      true,
		"summary": map[string]any{
			"from_disk_snapshot": true,
			"sync_status_line":   "Showing last known data",
		},
		"p2p": map[string]any{
			"from_disk_snapshot": true,
			"peer_dialing":       true,
			"dogego_sync_activity": map[string]any{
				"headline": "Connecting to peers",
			},
		},
	})
	out := sanitizeLiveJSONIfP2PActive(in)
	if len(out) == 0 {
		t.Fatal("peer_dialing disk bootstrap should promote sync activity")
	}
	var live map[string]any
	if err := json.Unmarshal(out, &live); err != nil {
		t.Fatal(err)
	}
	if live["from_disk_snapshot"] != nil || live["summary_stale"] != nil {
		t.Fatalf("envelope stale flags should clear: %#v", live)
	}
	sum, _ := live["summary"].(map[string]any)
	if sum["sync_status_line"] != "Connecting to peers" {
		t.Fatalf("sync_status_line=%v", sum["sync_status_line"])
	}
}
