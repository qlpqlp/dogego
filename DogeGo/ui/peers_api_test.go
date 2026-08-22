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
