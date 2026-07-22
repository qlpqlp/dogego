// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"time"

	"dogego/config"
	"dogego/rpc"
)

// CoreBip152ProbeResult is returned by GET /api/core-bip152-probe and embedded in /api/core-probes.
type CoreBip152ProbeResult struct {
	CheckedAt       string `json:"checked_at"`
	OK              bool   `json:"ok"`
	Skipped         bool   `json:"skipped,omitempty"`
	Reason          string `json:"reason,omitempty"`
	IBD             bool   `json:"ibd,omitempty"`
	PeerCount       int    `json:"peer_count"`
	HBToPeers       int    `json:"hb_to_peers"`
	HBFromPeers     int    `json:"hb_from_peers"`
	SchemaOK        bool   `json:"schema_ok"`
	HBNegotiated    bool   `json:"hb_negotiated"`
	CoreConfigured  bool   `json:"core_configured,omitempty"`
	CoreAvailable   bool   `json:"core_available,omitempty"`
	CorePeerCount   int    `json:"core_peer_count,omitempty"`
	CoreHBToPeers   int    `json:"core_hb_to_peers,omitempty"`
	CoreHBFromPeers int    `json:"core_hb_from_peers,omitempty"`
	CoreSchemaOK    bool   `json:"core_schema_ok,omitempty"`
	CoreHBNegotiated   bool              `json:"core_hb_negotiated,omitempty"`
	CmpctRelay         map[string]uint64 `json:"cmpct_relay,omitempty"`
	CmpctRelaySchemaOK bool              `json:"cmpct_relay_schema_ok,omitempty"`
	Issues          []string `json:"issues,omitempty"`
	Notes           []string `json:"notes,omitempty"`
	Hint            string `json:"hint,omitempty"`
}

type bip152PeerRow struct {
	addr   string
	hbTo   bool
	hbFrom bool
}

func peerInfoMapsFromAny(result any) ([]map[string]interface{}, bool) {
	switch v := result.(type) {
	case nil:
		return nil, true
	case []map[string]interface{}:
		return v, true
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				return nil, false
			}
			out = append(out, m)
		}
		return out, true
	default:
		return nil, false
	}
}

func parseGetPeerInfoBip152(result any) (rows []bip152PeerRow, schemaOK bool) {
	schemaOK = true
	maps, ok := peerInfoMapsFromAny(result)
	if !ok {
		schemaOK = false
		return nil, false
	}
	for _, m := range maps {
		toV, hasTo := m["bip152_hb_to"]
		fromV, hasFrom := m["bip152_hb_from"]
		if !hasTo || !hasFrom {
			schemaOK = false
		}
		row := bip152PeerRow{
			addr:   strFromAny(m["addr"]),
			hbTo:   boolFromAny(toV),
			hbFrom: boolFromAny(fromV),
		}
		rows = append(rows, row)
	}
	return rows, schemaOK
}

func summarizeBip152Peers(rows []bip152PeerRow) (hbTo, hbFrom int, negotiated bool) {
	for _, r := range rows {
		if r.hbTo {
			hbTo++
		}
		if r.hbFrom {
			hbFrom++
		}
	}
	return hbTo, hbFrom, hbTo > 0 || hbFrom > 0
}

// ProbeCoreBip152 checks DogeGo getpeerinfo BIP152 fields and optional Core side-by-side rows.
func ProbeCoreBip152(network string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) CoreBip152ProbeResult {
	ep := ResolveCoreParityEndpoints(network, conf)
	out := CoreBip152ProbeResult{
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		CoreConfigured: CoreCompareEnabled(network, conf),
		Hint:           "BIP152 v1 probe: getpeerinfo must expose bip152_hb_to/bip152_hb_from; when peers are connected, at least one link should negotiate HB (sendcmpct). getblockchaininfo exposes dogego_cmpct_* relay counters. AuxPoW blocks still use full inv/block.",
	}
	if invoke == nil {
		out.Skipped = true
		out.Reason = "dogego_rpc_not_ready"
		return out
	}
	info, err := invokeDogeGoRPC(invoke, "getblockchaininfo", nil)
	if err == nil {
		if ibd, ok := info["initialblockdownload"].(bool); ok {
			out.IBD = ibd
		}
		out.CmpctRelaySchemaOK = cmpctRelayCountersComplete(info)
		annotateCmpctRelayFromChainInfo(&out, info)
		if out.PeerCount > 0 && !out.IBD && !out.CmpctRelaySchemaOK {
			out.Issues = append(out.Issues, "cmpct_relay_counters_missing")
		}
	}
	peersRaw, err := invokeDogeGoRPCAny(invoke, "getpeerinfo", nil)
	if err != nil {
		out.Issues = append(out.Issues, "getpeerinfo_failed")
		out.Reason = err.Error()
		return out
	}
	rows, schemaOK := parseGetPeerInfoBip152(peersRaw)
	out.PeerCount = len(rows)
	out.SchemaOK = schemaOK
	out.HBToPeers, out.HBFromPeers, out.HBNegotiated = summarizeBip152Peers(rows)
	if !schemaOK {
		out.Issues = append(out.Issues, "getpeerinfo_missing_bip152_fields")
	}
	if out.PeerCount > 0 && !out.HBNegotiated && !out.IBD {
		out.Notes = append(out.Notes, "hb_not_negotiated_yet (compact relay may negotiate later)")
	}
	if out.PeerCount == 0 {
		out.Notes = append(out.Notes, "no_peers_connected")
	} else if out.HBNegotiated {
		out.Notes = append(out.Notes, fmt.Sprintf("hb_to=%d hb_from=%d on %d peer(s)", out.HBToPeers, out.HBFromPeers, out.PeerCount))
	} else if out.IBD {
		out.Notes = append(out.Notes, "IBD: HB may be deferred on ephemeral header-sync links")
	}

	if out.CoreConfigured {
		corePeers, coreErr := invokeExternalRPCAny(ep.Addr, ep.User, ep.Pass, "getpeerinfo", nil)
		if coreErr != nil {
			out.Notes = append(out.Notes, "core_unreachable: "+coreErr.Error())
		} else {
			out.CoreAvailable = true
			coreRows, coreSchema := parseGetPeerInfoBip152(corePeers)
			out.CorePeerCount = len(coreRows)
			out.CoreSchemaOK = coreSchema
			out.CoreHBToPeers, out.CoreHBFromPeers, out.CoreHBNegotiated = summarizeBip152Peers(coreRows)
			if !coreSchema {
				out.Notes = append(out.Notes, "core_getpeerinfo_missing_bip152_fields")
			}
			annotateBip152CoreParity(&out)
		}
	} else {
		out.Notes = append(out.Notes, "core_compare_optional")
	}

	out.OK = schemaOK && len(out.Issues) == 0
	if out.OK && out.PeerCount == 0 {
		out.Notes = append(out.Notes, "schema_ok_no_peers")
	}
	return out
}

func annotateBip152CoreParity(out *CoreBip152ProbeResult) {
	if out == nil || !out.CoreAvailable || out.IBD {
		return
	}
	if out.PeerCount == 0 || out.CorePeerCount == 0 {
		return
	}
	if out.HBNegotiated && out.CoreHBNegotiated {
		out.Notes = append(out.Notes, "hb_negotiated_dogego_and_core")
	} else if !out.HBNegotiated && !out.CoreHBNegotiated {
		out.Notes = append(out.Notes, "hb_not_negotiated_either_node")
	} else {
		out.Notes = append(out.Notes, "hb_negotiation_asymmetric_vs_core")
	}
}

func cmpctRelayCountersComplete(info map[string]interface{}) bool {
	if info == nil {
		return false
	}
	for _, k := range rpc.DogegoCmpctRelayCounterKeys() {
		if _, ok := uint64FromAny(info[k]); !ok {
			return false
		}
	}
	return true
}

func annotateCmpctRelayFromChainInfo(out *CoreBip152ProbeResult, info map[string]interface{}) {
	if out == nil || info == nil {
		return
	}
	relay := map[string]uint64{}
	for _, k := range rpc.DogegoCmpctRelayCounterKeys() {
		if v, ok := uint64FromAny(info[k]); ok {
			relay[k] = v
		}
	}
	if len(relay) == 0 {
		return
	}
	out.CmpctRelay = relay
	if out.IBD || !out.HBNegotiated || out.PeerCount == 0 {
		return
	}
	active := relay["dogego_cmpct_reconstruct_ok"] > 0 ||
		relay["dogego_cmpct_announced_out"] > 0 ||
		relay["dogego_cmpct_served_getdata"] > 0
	if active {
		out.Notes = append(out.Notes, "cmpct_relay_active")
	} else {
		out.Notes = append(out.Notes, "cmpct_relay_idle")
	}
	if relay["dogego_cmpct_fallback_full_block"] > 0 || relay["dogego_cmpct_reconstruct_fallback_getdata"] > 0 {
		out.Notes = append(out.Notes, "cmpct_full_block_fallback_seen (AuxPoW-era or reconstruct miss)")
	}
}

func uint64FromAny(v any) (uint64, bool) {
	if v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case uint64:
		return n, true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case float64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	default:
		return 0, false
	}
}
