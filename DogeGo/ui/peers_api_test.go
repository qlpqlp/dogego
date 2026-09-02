// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestBuildPeersDashboardResponse(t *testing.T) {
	cfg := StartConfig{
		RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
			switch method {
			case "getpeerinfo":
				return map[string]interface{}{
					"result": []map[string]interface{}{
						{"id": 1, "addr": "1.2.3.4:44556", "inbound": false, "dogego_role": "primary"},
						{"id": 2, "addr": "5.6.7.8:44556", "inbound": true, "dogego_relay_cgnat": true},
					},
				}
			case "getaddednodeinfo":
				return map[string]interface{}{
					"result": []map[string]interface{}{
						{"addednode": "10.0.0.5:44556", "connected": true},
					},
				}
			default:
				t.Fatalf("method %q", method)
				return nil
			}
		},
		P2PSnapshot: func() map[string]any {
			return map[string]any{"p2p_connectivity": "cgnat", "connections_total": 2}
		},
		DGRSnapshot: func() map[string]any {
			return map[string]any{"enabled": true, "using_relay": true, "active_relay": "relay.example:24433"}
		},
	}
	out := BuildPeersDashboardResponse(cfg)
	if out["ok"] != true {
		t.Fatalf("ok=%v err=%v", out["ok"], out["error"])
	}
	if out["connections_inbound"] != 1 || out["connections_outbound"] != 1 {
		t.Fatalf("counts in=%v out=%v", out["connections_inbound"], out["connections_outbound"])
	}
	added := out["added_nodes"]
	switch v := added.(type) {
	case []map[string]interface{}:
		if len(v) != 1 {
			t.Fatalf("added_nodes=%#v", added)
		}
	case []interface{}:
		if len(v) != 1 {
			t.Fatalf("added_nodes=%#v", added)
		}
	default:
		t.Fatalf("added_nodes=%#v", added)
	}
	if out["p2p"] == nil || out["dgr"] == nil {
		t.Fatalf("missing p2p/dgr context: %#v", out)
	}
}

func TestBuildPeersDashboardPrefersP2PSnapCountsOverPeerList(t *testing.T) {
	// Many getpeerinfo rows (cooling/relay) must not override dock/header sync counts.
	var peers []map[string]interface{}
	for i := 0; i < 51; i++ {
		peers = append(peers, map[string]interface{}{
			"id": i + 1, "addr": fmt.Sprintf("10.0.0.%d:22556", i+1), "inbound": false,
		})
	}
	cfg := StartConfig{
		RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
			switch method {
			case "getpeerinfo":
				return map[string]interface{}{"result": peers}
			case "getaddednodeinfo":
				return map[string]interface{}{"result": []any{}}
			default:
				t.Fatalf("method %q", method)
				return nil
			}
		},
		P2PSnapshot: func() map[string]any {
			return map[string]any{
				"connections_outbound": 18,
				"connections_inbound":  0,
				"connections_total":    18,
			}
		},
	}
	out := BuildPeersDashboardResponse(cfg)
	if out["connections_outbound"] != 18 || out["connections_inbound"] != 0 {
		t.Fatalf("want snap 18/0, got out=%v in=%v (list had %d)", out["connections_outbound"], out["connections_inbound"], len(peers))
	}
	if peerListLen(out["peers"]) != 51 {
		t.Fatalf("peer cards should still list getpeerinfo rows, got %d", peerListLen(out["peers"]))
	}
}

func TestApplyPeersActionAdd(t *testing.T) {
	var sawMethod string
	var sawParams []json.RawMessage
	cfg := StartConfig{
		RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
			if method == "addnode" {
				sawMethod = method
				sawParams = params
				return map[string]interface{}{"result": nil}
			}
			if method == "getpeerinfo" {
				return map[string]interface{}{"result": []any{}}
			}
			if method == "getaddednodeinfo" {
				return map[string]interface{}{"result": []any{}}
			}
			t.Fatalf("unexpected %s", method)
			return nil
		},
	}
	out, code := applyPeersAction(cfg, "add", "192.168.1.10:44556")
	if code != 200 || out["ok"] != true {
		t.Fatalf("code=%d out=%v", code, out)
	}
	if sawMethod != "addnode" || len(sawParams) != 2 {
		t.Fatalf("rpc %s %#v", sawMethod, sawParams)
	}
}

func TestBuildPeersDashboardResponseTimesOutGetPeerInfo(t *testing.T) {
	cfg := StartConfig{
		RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
			if method == "getpeerinfo" {
				time.Sleep(4 * time.Second)
				return map[string]interface{}{
					"result": []map[string]interface{}{
						{"id": 1, "addr": "1.2.3.4:44556", "inbound": false},
					},
				}
			}
			if method == "getaddednodeinfo" {
				return map[string]interface{}{"result": []any{}}
			}
			t.Fatalf("method %q", method)
			return nil
		},
		P2PSnapshot: func() map[string]any {
			return map[string]any{
				"connections_outbound": 4,
				"connections_inbound":  1,
				"connections_total":    5,
			}
		},
	}
	start := time.Now()
	out := BuildPeersDashboardResponse(cfg)
	if time.Since(start) > 3500*time.Millisecond {
		t.Fatalf("peers API took too long under getpeerinfo hang: %s", time.Since(start))
	}
	if out["peers_partial"] != true {
		t.Fatalf("expected peers_partial: %#v", out)
	}
	if out["connections_outbound"] != 4 {
		t.Fatalf("expected P2P fallback counts: %#v", out)
	}
	if out["ok"] != true {
		t.Fatalf("ok=%v err=%v", out["ok"], out["error"])
	}
}

func TestBuildPeersDashboardResponseFallsBackToP2PCounts(t *testing.T) {
	cfg := StartConfig{
		RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
			switch method {
			case "getpeerinfo":
				return map[string]interface{}{"result": nil}
			case "getaddednodeinfo":
				return map[string]interface{}{"result": []any{}}
			default:
				t.Fatalf("method %q", method)
				return nil
			}
		},
		P2PSnapshot: func() map[string]any {
			return map[string]any{
				"connections_outbound": 8,
				"connections_inbound":  1,
				"connections_total":    9,
			}
		},
	}
	out := BuildPeersDashboardResponse(cfg)
	if out["connections_outbound"] != 8 || out["connections_inbound"] != 1 {
		t.Fatalf("fallback counts: %#v", out)
	}
	if out["connections_total"] != 9 {
		t.Fatalf("total=%v", out["connections_total"])
	}
}

func TestBuildPeersDashboardSynthesizesAssistPeers(t *testing.T) {
	cfg := StartConfig{
		RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
			switch method {
			case "getpeerinfo":
				return map[string]interface{}{"result": []any{}}
			case "getaddednodeinfo":
				return map[string]interface{}{"result": []any{}}
			default:
				t.Fatalf("method %q", method)
				return nil
			}
		},
		P2PSnapshot: func() map[string]any {
			return map[string]any{
				"connections_outbound":     0, // disk-zeroed; heal from assist
				"block_assist_connections": 2,
				"primary_peer":             "10.0.0.1:22556",
				"block_assist_peers": []map[string]any{
					{"addr": "10.0.0.2:22556", "lane": 1, "bytes_recv": 100, "bytes_sent": 10},
					{"addr": "10.0.0.3:22556", "lane": 2, "bytes_recv": 200, "bytes_sent": 20},
				},
			}
		},
	}
	out := BuildPeersDashboardResponse(cfg)
	if peerListLen(out["peers"]) != 3 {
		t.Fatalf("want 3 synthesized peers, got %#v", out["peers"])
	}
	if out["connections_outbound"] != 3 {
		t.Fatalf("outbound=%v want 3", out["connections_outbound"])
	}
	if out["peers_from_p2p"] != true || out["ok"] != true {
		t.Fatalf("flags %#v", out)
	}
}

func TestBuildPeersDashboardIgnoresDiskSnapshotAssist(t *testing.T) {
	cfg := StartConfig{
		RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
			switch method {
			case "getpeerinfo":
				return map[string]interface{}{"result": []any{}}
			case "getaddednodeinfo":
				return map[string]interface{}{"result": []any{}}
			default:
				t.Fatalf("method %q", method)
				return nil
			}
		},
		P2PSnapshot: func() map[string]any {
			return map[string]any{
				"from_disk_snapshot":       true,
				"connections_outbound":     0,
				"block_assist_connections": 2,
				"primary_peer":             "10.0.0.1:22556",
				"block_assist_peers": []map[string]any{
					{"addr": "10.0.0.2:22556"},
				},
			}
		},
	}
	out := BuildPeersDashboardResponse(cfg)
	if peerListLen(out["peers"]) != 0 {
		t.Fatalf("disk snapshot must not synthesize peers, got %#v", out["peers"])
	}
	if out["connections_outbound"] != 0 {
		t.Fatalf("outbound=%v want 0", out["connections_outbound"])
	}
}
