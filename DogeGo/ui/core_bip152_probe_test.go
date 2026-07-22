// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"testing"

	"dogego/config"
	"dogego/rpc"
)

func TestParseGetPeerInfoBip152(t *testing.T) {
	raw := []interface{}{
		map[string]interface{}{
			"addr":            "1.2.3.4:22556",
			"bip152_hb_to":    true,
			"bip152_hb_from":  false,
		},
		map[string]interface{}{
			"addr": "5.6.7.8:22556",
		},
	}
	rows, schemaOK := parseGetPeerInfoBip152(raw)
	if schemaOK {
		t.Fatal("expected schema failure on missing fields")
	}
	if len(rows) != 2 || !rows[0].hbTo || rows[0].hbFrom || rows[1].hbTo {
		t.Fatalf("rows: %+v", rows)
	}

	t.Run("in_process_rpc_slice", func(t *testing.T) {
		inProc := []map[string]interface{}{
			{
				"addr":           "peer:1",
				"bip152_hb_to":   true,
				"bip152_hb_from": false,
			},
		}
		got, ok := parseGetPeerInfoBip152(inProc)
		if !ok || len(got) != 1 || !got[0].hbTo || got[0].hbFrom {
			t.Fatalf("in-process slice: ok=%v rows=%+v", ok, got)
		}
	})
}

func TestProbeCoreBip152_negotiatedOK(t *testing.T) {
	cmpctInfo := map[string]interface{}{"initialblockdownload": false}
	for _, k := range rpc.DogegoCmpctRelayCounterKeys() {
		cmpctInfo[k] = float64(0)
	}
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": cmpctInfo}
		case "getpeerinfo":
			return map[string]interface{}{
				"result": []map[string]interface{}{
					{
						"addr":           "peer:1",
						"bip152_hb_to":   true,
						"bip152_hb_from": true,
					},
				},
			}
		default:
			return map[string]interface{}{"error": map[string]interface{}{"message": "unknown"}}
		}
	}
	out := ProbeCoreBip152("testnet", config.File{}, invoke)
	if !out.OK || !out.SchemaOK || !out.HBNegotiated || out.HBToPeers != 1 || out.HBFromPeers != 1 {
		t.Fatalf("probe: %+v", out)
	}
	if !out.CmpctRelaySchemaOK {
		t.Fatalf("cmpct schema: %+v", out)
	}
}

func TestProbeCoreBip152_noHBWhenCaughtUpFails(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{"initialblockdownload": false}}
		case "getpeerinfo":
			return map[string]interface{}{
				"result": []interface{}{
					map[string]interface{}{
						"addr":           "peer:1",
						"bip152_hb_to":   false,
						"bip152_hb_from": false,
					},
				},
			}
		default:
			return map[string]interface{}{"error": map[string]interface{}{"message": "unknown"}}
		}
	}
	out := ProbeCoreBip152("testnet", config.File{}, invoke)
	if !out.OK || !out.SchemaOK {
		t.Fatalf("expected schema-ok pass when HB not negotiated yet: %+v", out)
	}
	if len(out.Issues) > 0 {
		t.Fatalf("unexpected issues: %+v", out.Issues)
	}
}

func TestProbeCoreBip152_ibdWithEphemeralPeersPasses(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{"initialblockdownload": true}}
		case "getpeerinfo":
			return map[string]interface{}{
				"result": []map[string]interface{}{
					{
						"addr":           "assist:1",
						"bip152_hb_to":   false,
						"bip152_hb_from": false,
						"dogego_role":    "block-assist",
					},
				},
			}
		default:
			return map[string]interface{}{"error": map[string]interface{}{"message": "unknown"}}
		}
	}
	out := ProbeCoreBip152("testnet", config.File{}, invoke)
	if !out.OK || !out.SchemaOK || out.PeerCount != 1 || !out.IBD {
		t.Fatalf("expected IBD pass with schema ok: %+v", out)
	}
}

func TestProbeCoreBip152_noInvokeSkipped(t *testing.T) {
	out := ProbeCoreBip152("testnet", config.File{}, nil)
	if !out.Skipped || out.OK {
		t.Fatalf("expected skipped: %+v", out)
	}
}

func TestAnnotateBip152CoreParity_bothNegotiated(t *testing.T) {
	out := CoreBip152ProbeResult{
		CoreAvailable: true, IBD: false,
		PeerCount: 1, CorePeerCount: 1,
		HBNegotiated: true, CoreHBNegotiated: true,
	}
	annotateBip152CoreParity(&out)
	if len(out.Notes) != 1 || out.Notes[0] != "hb_negotiated_dogego_and_core" {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestAnnotateBip152CoreParity_asymmetric(t *testing.T) {
	out := CoreBip152ProbeResult{
		CoreAvailable: true, IBD: false,
		PeerCount: 1, CorePeerCount: 1,
		HBNegotiated: true, CoreHBNegotiated: false,
	}
	annotateBip152CoreParity(&out)
	if len(out.Notes) != 1 || out.Notes[0] != "hb_negotiation_asymmetric_vs_core" {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestAnnotateCmpctRelayFromChainInfo_active(t *testing.T) {
	out := CoreBip152ProbeResult{IBD: false, HBNegotiated: true, PeerCount: 1}
	info := map[string]interface{}{
		"dogego_cmpct_in":             float64(3),
		"dogego_cmpct_reconstruct_ok": float64(2),
		"dogego_cmpct_announced_out":  float64(1),
	}
	annotateCmpctRelayFromChainInfo(&out, info)
	if out.CmpctRelay["dogego_cmpct_reconstruct_ok"] != 2 {
		t.Fatalf("relay=%v", out.CmpctRelay)
	}
	found := false
	for _, n := range out.Notes {
		if n == "cmpct_relay_active" {
			found = true
		}
	}
	if !found {
		t.Fatalf("notes=%v", out.Notes)
	}
}
