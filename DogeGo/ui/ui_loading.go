// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "dogego/store"

// ApplyUILoadingFlags sets dogego_ui_loading* fields for the sync dock / Overview.
// Background body reconcile never steals the dock once IBD/connect/peers are live — it only
// attaches dogego_disk_verify_* notes so peer counts and download rates stay visible.
func ApplyUILoadingFlags(summary map[string]any, warming bool) {
	if summary == nil {
		return
	}
	attachContiguousReconcileNote(summary)
	live := summaryHasLiveSyncWork(summary)
	if live {
		if summary["dogego_ui_loading_phase"] == "body_reconcile" {
			clearUILoading(summary)
		}
		delete(summary, "warming_up")
	} else if applyContiguousReconcileLoading(summary) {
		return
	} else if warming {
		setUILoading(summary, "warming", "Loading local data…")
		return
	}

	downloadActive := syncDownloadActive(summary)
	if lag, ok := int64FromAny(summary["dogego_connect_lag"]); ok && lag > 64 && !downloadActive {
		setUILoading(summary, "utxo_cache", "Connecting blocks…")
		return
	}
	if lag, ok := int64FromAny(summary["dogego_stored_bodies_ahead_connect"]); ok && lag > 64 && !downloadActive {
		setUILoading(summary, "snapshot_replay", "Connecting stored bodies…")
		return
	}
	ibd := summary["initialblockdownload"] == true || summary["ibd_active"] == true
	if !ibd && (summary["scanning"] == true || summary["wallet_listtransactions_scan_pending"] == true) {
		setUILoading(summary, "wallet_scan", "Scanning wallet history…")
		return
	}
	if !ibd && summary["needs_rescan"] == true && summary["wallet_scan_index_ok"] != true {
		setUILoading(summary, "wallet_scan", "Building wallet scan index…")
		return
	}
	// Body/header download owns the dock — drop sticky warming/reconcile/connect loading.
	clearUILoading(summary)
}

// attachContiguousReconcileNote publishes soft verify progress without owning the dock.
func attachContiguousReconcileNote(summary map[string]any) {
	st, ok := store.ContiguousReconcileProgress()
	if !ok || !st.Active {
		delete(summary, "dogego_disk_verify_detail")
		delete(summary, "dogego_disk_verify_pct")
		return
	}
	detail := st.Detail
	if detail == "" {
		detail = "Checking stored blocks…"
	}
	summary["dogego_disk_verify_detail"] = detail
	if st.Percent >= 0 {
		summary["dogego_disk_verify_pct"] = st.Percent
	} else {
		delete(summary, "dogego_disk_verify_pct")
	}
}

// applyContiguousReconcileLoading surfaces startup blk*/body reconcile progress on the dock
// only when nothing else is the primary sync story yet.
func applyContiguousReconcileLoading(summary map[string]any) bool {
	st, ok := store.ContiguousReconcileProgress()
	if !ok || !st.Active {
		delete(summary, "dogego_ui_loading_pct")
		return false
	}
	if summaryHasLiveSyncWork(summary) {
		return false
	}
	detail := st.Detail
	if detail == "" {
		detail = "Checking stored blocks…"
	}
	setUILoading(summary, "body_reconcile", detail)
	summary["warming_up"] = true
	summary["sync_status_line"] = detail
	summary["dogego_sync_status"] = detail
	if st.Percent >= 0 {
		summary["dogego_ui_loading_pct"] = st.Percent
	} else {
		delete(summary, "dogego_ui_loading_pct")
	}
	return true
}

func summaryHasLiveSyncWork(summary map[string]any) bool {
	if summary == nil {
		return false
	}
	if out, ok := int64FromAny(summary["connections_out"]); ok && out > 0 {
		return true
	}
	if inn, ok := int64FromAny(summary["connections_in"]); ok && inn > 0 {
		return true
	}
	if bpm, ok := float64FromAny(summary["blocks_per_minute"]); ok && bpm > 0 {
		return true
	}
	if bpm, ok := float64FromAny(summary["contiguous_blocks_per_minute"]); ok && bpm > 0 {
		return true
	}
	if lag, ok := int64FromAny(summary["dogego_connect_lag"]); ok && lag > 0 {
		return true
	}
	if summary["dogego_sync_activity"] != nil {
		return true
	}
	return syncDownloadActive(summary)
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
	delete(summary, "dogego_ui_loading_pct")
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
