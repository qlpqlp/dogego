// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "testing"

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
