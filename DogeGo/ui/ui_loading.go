// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// ApplyUILoadingFlags sets dogego_ui_loading* fields for the sync dock / Overview.
// Priority: warming â†’ connect/replay (only when download is not the main work) â†’ wallet_scan.
// During body/header IBD, connect lag is background work â€” do not sticky-override the dock
// with "Building UTXO cache" (operators already see connect lag in the metrics strip).
func ApplyUILoadingFlags(summary map[string]any, warming bool) {
	if summary == nil {
		return
	}
	if warming {
		setUILoading(summary, "warming", "Loading local dataâ€¦")
		return
	}
	downloadActive := syncDownloadActive(summary)
	if lag, ok := int64FromAny(summary["dogego_connect_lag"]); ok && lag > 64 && !downloadActive {
		setUILoading(summary, "utxo_cache", "Connecting blocksâ€¦")
		return
	}
	if lag, ok := int64FromAny(summary["dogego_stored_bodies_ahead_connect"]); ok && lag > 64 && !downloadActive {
		setUILoading(summary, "snapshot_replay", "Connecting stored bodiesâ€¦")
		return
	}
	ibd := summary["initialblockdownload"] == true || summary["ibd_active"] == true
	if !ibd && (summary["scanning"] == true || summary["wallet_listtransactions_scan_pending"] == true) {
		setUILoading(summary, "wallet_scan", "Scanning wallet historyâ€¦")
		return
	}
	if !ibd && summary["needs_rescan"] == true && summary["wallet_scan_index_ok"] != true {
		setUILoading(summary, "wallet_scan", "Building wallet scan indexâ€¦")
		return
	}
	clearUILoading(summary)
}

// syncDownloadActive reports headers/body download as the operator-visible primary sync work.
// Connect catch-up may run in parallel; the dock should keep download/IBD phase labels then.
func syncDownloadActive(summary map[string]any) bool {
	if summary == nil {
		return false
	}
	if summary["dogego_genesis_missing"] == true {
		return true
	}
	if summary["headers_syncing"] == true || summary["dogego_header_catch_up_pending"] == true {
		return true
	}
	if behind, ok := int64FromAny(summary["blocks_behind_headers"]); ok && behind > 32 {
		return true
	}
	if vp, ok := float64FromAny(summary["dogego_body_verification_progress"]); ok && vp < 0.999 {
		return true
	}
	return false
}

func setUILoading(summary map[string]any, phase, detail string) {
	summary["dogego_ui_loading"] = true
	summary["dogego_ui_loading_phase"] = phase
	summary["dogego_ui_loading_detail"] = detail
}

func clearUILoading(summary map[string]any) {
	delete(summary, "dogego_ui_loading")
	delete(summary, "dogego_ui_loading_phase")
	delete(summary, "dogego_ui_loading_detail")
}

func int64FromAny(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

func float64FromAny(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
