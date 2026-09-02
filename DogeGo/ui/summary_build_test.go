// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"testing"

	"dogego/wallet"
)

func TestMergeIBDProgressConnectCatchUpTuning(t *testing.T) {
	summary := map[string]any{}
	mergeIBDProgressDiagnostics(summary, map[string]interface{}{
		"connect_lag":                int64(9000),
		"connect_blocks_per_minute":  42.5,
		"connect_catch_up_min_lag":   int64(512),
		"connect_catch_up_passes":    8,
		"connect_catch_up_batch":     128,
		"connect_catch_up_interval_ms": int64(500),
	})
	if got := summary["dogego_connect_lag"]; got != int64(9000) {
		t.Fatalf("connect_lag=%v want 9000", got)
	}
	if got := summary["dogego_connect_catch_up_passes"]; got != 8 {
		t.Fatalf("passes=%v want 8", got)
	}
	if got := summary["dogego_connect_catch_up_batch"]; got != 128 {
		t.Fatalf("batch=%v want 128", got)
	}
	if got := summary["dogego_connect_catch_up_interval_ms"]; got != int64(500) {
		t.Fatalf("interval_ms=%v want 500", got)
	}
	if got := summary["dogego_connect_catch_up_min_lag"]; got != int64(512) {
		t.Fatalf("min_lag=%v want 512", got)
	}
}

func TestBuildSummaryMapWalletRescanFields(t *testing.T) {
	cfg, _, spk := testWalletFastSetup(t)
	addWalletFastUtxo(cfg.UtxoCache(), 9, 0, 1_000_000_000, 300, spk)
	cfg.ActiveWallet().SeedScannedTx([]wallet.ScannedTx{{
		TxID: "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		Category: "receive", Address: cfg.ActiveWallet().Address(), AmountKoinu: 1_000_000_000,
		BlockHeight: 250, Vout: 0,
	}})
	summary := map[string]any{}
	attachWalletRescanStatus(summary, cfg)
	if summary["needs_rescan"] != true {
		t.Fatalf("needs_rescan=%v want true for partial index", summary["needs_rescan"])
	}
	if summary["wallet_scan_index_ok"] == true {
		t.Fatalf("wallet_scan_index_ok=%v want false when index lags tip", summary["wallet_scan_index_ok"])
	}
	if summary["wallet_history_fast_path"] != true {
		t.Fatalf("wallet_history_fast_path=%v want true", summary["wallet_history_fast_path"])
	}
}

func TestSummaryWalletHistoryDeferred(t *testing.T) {
	cfg, _, _ := testWalletFastSetup(t)
	cfg.RPCInvoke = func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"initialblockdownload": false,
				"dogego_connect_lag":   float64(128),
			}}
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{"balance": float64(1)}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	summary := map[string]any{}
	attachWalletRescanStatus(summary, cfg)
	attachWalletHistoryDeferStatus(summary, cfg)
	if summary["wallet_history_deferred"] != true {
		t.Fatalf("wallet_history_deferred=%v", summary["wallet_history_deferred"])
	}
	if summary["wallet_history_defer_reason"] != "connect_lag" {
		t.Fatalf("wallet_history_defer_reason=%v", summary["wallet_history_defer_reason"])
	}
}

func TestMergeIBDProgressBodyEta(t *testing.T) {
	summary := map[string]any{}
	mergeIBDProgressDiagnostics(summary, map[string]interface{}{
		"body_ibd_eta_minutes": int64(43_524),
	})
	if got := summary["dogego_body_ibd_eta_minutes"]; got != int64(43_524) {
		t.Fatalf("body eta=%v want 43524", got)
	}
}

func TestMergeP2PSummaryAddrbookFields(t *testing.T) {
	summary := map[string]any{}
	p2pSnap := map[string]any{
		"addrbook_tried":                 42,
		"addrbook_new":                   100,
		"addrbook_n_key_set":             true,
		"addrbook_bucket_slot_cap":       64,
		"addrbook_tried_bucket_max_fill": 12,
	}
	mergeP2PSummaryExtraFields(summary, p2pSnap)
	if summary["addrbook_tried"] != 42 {
		t.Fatalf("addrbook_tried=%v", summary["addrbook_tried"])
	}
	if summary["addrbook_new"] != 100 {
		t.Fatalf("addrbook_new=%v", summary["addrbook_new"])
	}
	if summary["addrbook_n_key_set"] != true {
		t.Fatalf("addrbook_n_key_set=%v", summary["addrbook_n_key_set"])
	}
}
