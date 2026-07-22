// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestMergeDogegoRawSyncDiagnostics(t *testing.T) {
	res := map[string]interface{}{}
	prog := map[string]interface{}{
		"block_stalling_timeout_sec":        int64(2),
		"block_download_timeout_sec":        int64(300),
		"last_block_stall_peer":             "1.2.3.4:22556",
		"last_block_stall_at":               int64(100),
		"last_block_download_timeout_peer":  "5.6.7.8:22556",
		"last_block_download_timeout_at":    int64(200),
		"frontier_stalling_since":           int64(99),
		"max_blocks_in_transit_per_peer":    16,
		"lane_in_flight":                    map[string]int{"1.2.3.4:22556": 8},
		"raw_blocks_ahead_of_contiguous":    int64(28),
		"connect_lag":                       int64(5200),
		"connect_catch_up_min_lag":          int64(32),
		"connect_catch_up_passes":           8,
		"connect_catch_up_batch":            128,
		"connect_catch_up_interval_ms":      int64(500),
		"body_ibd_header_paused":            true,
		"body_ibd_eta_minutes":              int64(43_524),
	}
	mergeDogegoRawSyncDiagnostics(res, prog)
	if res["dogego_last_block_stall_peer"] != "1.2.3.4:22556" {
		t.Fatalf("stall peer %v", res["dogego_last_block_stall_peer"])
	}
	if res["dogego_block_download_timeout_sec"] != int64(300) {
		t.Fatalf("timeout sec %v", res["dogego_block_download_timeout_sec"])
	}
	if res["dogego_max_blocks_in_transit_per_peer"] != 16 {
		t.Fatalf("in-transit cap %v", res["dogego_max_blocks_in_transit_per_peer"])
	}
	lanes, _ := res["dogego_lane_in_flight"].(map[string]int)
	if lanes["1.2.3.4:22556"] != 8 {
		t.Fatalf("lane in flight %v", res["dogego_lane_in_flight"])
	}
	if res["dogego_raw_blocks_ahead_of_contiguous"] != int64(28) {
		t.Fatalf("ahead-of-contiguous %v", res["dogego_raw_blocks_ahead_of_contiguous"])
	}
	if res["dogego_connect_lag"] != int64(5200) {
		t.Fatalf("connect lag %v", res["dogego_connect_lag"])
	}
	if res["dogego_connect_catch_up_min_lag"] != int64(32) {
		t.Fatalf("min lag %v", res["dogego_connect_catch_up_min_lag"])
	}
	if res["dogego_connect_catch_up_passes"] != 8 {
		t.Fatalf("passes %v", res["dogego_connect_catch_up_passes"])
	}
	if res["dogego_connect_catch_up_batch"] != 128 {
		t.Fatalf("batch %v", res["dogego_connect_catch_up_batch"])
	}
	if res["dogego_connect_catch_up_interval_ms"] != int64(500) {
		t.Fatalf("interval %v", res["dogego_connect_catch_up_interval_ms"])
	}
	if res["dogego_body_ibd_header_paused"] != true {
		t.Fatalf("body pause %v", res["dogego_body_ibd_header_paused"])
	}
	if res["dogego_body_ibd_eta_minutes"] != int64(43_524) {
		t.Fatalf("body eta %v", res["dogego_body_ibd_eta_minutes"])
	}
}
