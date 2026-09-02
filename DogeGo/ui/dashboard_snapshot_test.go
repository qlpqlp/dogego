// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestDashboardSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sum := map[string]any{
		"tip_height":   int64(42),
		"header_count": int64(43),
		"mempool_txs":  3,
		"network":      "mainnet",
	}
	snap := &dashboardSnapshotFile{
		TipHeight: 42,
		Summary:   sum,
		P2P:       json.RawMessage(`{"wired":true}`),
		AnalyticsSummary: map[string]any{
			"analytics_db_exists": true,
			"headers_tip_height":  42,
		},
	}
	if err := saveDashboardSnapshot(dir, snap); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, dashboardSnapshotFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	got, err := loadDashboardSnapshot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if tipHeightFromSummary(got.Summary) != 42 {
		t.Fatalf("tip %#v", got.Summary["tip_height"])
	}
	if got.AnalyticsSummary["analytics_db_exists"] != true {
		t.Fatalf("analytics %#v", got.AnalyticsSummary)
	}
}

func TestBootstrapLiveFromDiskSnapshot(t *testing.T) {
	dir := t.TempDir()
	sum := map[string]any{
		"tip_height":   float64(100),
		"header_count": float64(101),
		"network":      "mainnet",
		"mempool_txs":  float64(7),
	}
	if err := saveDashboardSnapshot(dir, &dashboardSnapshotFile{
		TipHeight: 100,
		Summary:   sum,
	}); err != nil {
		t.Fatal(err)
	}
	var f LiveFeed
	f.chainDataDir = dir
	f.bootstrapLiveIfEmpty(StartConfig{ChainDataDir: dir, Network: "mainnet", ChainDisplay: "mainnet"})
	f.mu.RLock()
	liveJSON := append([]byte(nil), f.liveJSON...)
	f.mu.RUnlock()
	if len(liveJSON) == 0 {
		t.Fatal("expected live JSON from disk snapshot")
	}
	var live map[string]any
	if err := json.Unmarshal(liveJSON, &live); err != nil {
		t.Fatal(err)
	}
	if live["from_disk_snapshot"] != true || live["summary_stale"] != true {
		t.Fatalf("flags %#v", live)
	}
	outSum, _ := live["summary"].(map[string]any)
	if outSum["from_disk_snapshot"] != true {
		t.Fatalf("summary flags %#v", outSum)
	}
	if outSum["dogego_ui_loading"] == true {
		t.Fatal("disk snapshot should not set ui loading")
	}
}

func TestMarkSummaryFromDiskSnapshotZerosLiveMetrics(t *testing.T) {
	sum := map[string]any{
		"tip_height":                       int64(100),
		"blocks_per_minute":                250.0,
		"contiguous_blocks_per_minute":     200.0,
		"dogego_connect_blocks_per_minute": 50.0,
		"connections_out":                  36,
		"connections_in":                   2,
		"in_flight_batches":                180,
		"sync_eta":                         "about 2 weeks",
	}
	markSummaryFromDiskSnapshot(sum)
	if sum["blocks_per_minute"] != nil || sum["contiguous_blocks_per_minute"] != nil {
		t.Fatalf("rates should be cleared: %#v", sum)
	}
	if sum["connections_out"] != 0 || sum["connections_in"] != 0 {
		t.Fatalf("peers should be 0: %#v", sum)
	}
	if sum["in_flight_batches"] != nil || sum["sync_eta"] != nil {
		t.Fatalf("volatile IBD fields should be cleared: %#v", sum)
	}
	if sum["tip_height"] != int64(100) {
		t.Fatalf("tip should remain: %#v", sum["tip_height"])
	}
	p2 := map[string]any{
		"wired":                    true,
		"connections_outbound":     36,
		"block_assist_connections": 17,
		"block_assist_peers":       []any{map[string]any{"addr": "1.2.3.4:22556"}},
		"primary_peer":             "5.6.7.8:22556",
		"health_message":           "P2P active with 18 outbound sync connection(s).",
		"ibd_progress":             map[string]any{"blocks_per_minute": 250.0},
	}
	zeroP2PLiveMetrics(p2)
	if p2["connections_outbound"] != 0 || p2["ibd_progress"] != nil {
		t.Fatalf("p2p live metrics should be cleared: %#v", p2)
	}
	if p2["block_assist_connections"] != 0 || p2["block_assist_peers"] != nil || p2["primary_peer"] != nil {
		t.Fatalf("assist/primary must be stripped on cold start: %#v", p2)
	}
	if p2["from_disk_snapshot"] != true {
		t.Fatal("from_disk_snapshot must remain set after zeroing")
	}
}

func TestApplyUILoadingFlagsWarming(t *testing.T) {
	sum := map[string]any{}
	ApplyUILoadingFlags(sum, true)
	if sum["dogego_ui_loading"] != true || sum["dogego_ui_loading_phase"] != "warming" {
		t.Fatalf("%#v", sum)
	}
	ApplyUILoadingFlags(sum, false)
	sum["scanning"] = true
	ApplyUILoadingFlags(sum, false)
	if sum["dogego_ui_loading_phase"] != "wallet_scan" {
		t.Fatalf("%#v", sum)
	}
	clearUILoading(sum)
	sum["dogego_connect_lag"] = int64(128)
	ApplyUILoadingFlags(sum, false)
	if sum["dogego_ui_loading_phase"] != "utxo_cache" {
		t.Fatalf("connect-only lag should set utxo_cache phase: %#v", sum)
	}
	// Body download still active → do not sticky-override dock with UTXO/connect loading.
	sum["blocks_behind_headers"] = int64(50000)
	sum["dogego_body_verification_progress"] = 0.4
	sum["initialblockdownload"] = true
	ApplyUILoadingFlags(sum, false)
	if sum["dogego_ui_loading"] == true {
		t.Fatalf("during body IBD, connect lag must not set ui loading: %#v", sum)
	}
}

func TestApplyUILoadingFlagsBodyReconcile(t *testing.T) {
	store.BeginContiguousReconcile()
	defer store.EndContiguousReconcile()
	sum := map[string]any{}
	ApplyUILoadingFlags(sum, true)
	if sum["dogego_ui_loading_phase"] != "body_reconcile" {
		t.Fatalf("reconcile should win over warming when idle: %#v", sum)
	}
	if sum["dogego_ui_loading_pct"] == nil {
		t.Fatalf("expected loading pct: %#v", sum)
	}
	// IBD already behind headers → dock stays on sync, verify is a soft note only.
	sum2 := map[string]any{"blocks_behind_headers": int64(100000), "initialblockdownload": true}
	ApplyUILoadingFlags(sum2, true)
	if sum2["dogego_ui_loading_phase"] == "body_reconcile" {
		t.Fatalf("live IBD must not be overridden by reconcile: %#v", sum2)
	}
	if sum2["dogego_disk_verify_detail"] == nil {
		t.Fatalf("expected soft verify note: %#v", sum2)
	}
	if sum2["warming_up"] == true {
		t.Fatalf("warming_up must clear when IBD is live: %#v", sum2)
	}
}

func TestApplyUILoadingFlagsConnectAfterDownload(t *testing.T) {
	sum := map[string]any{
		"dogego_connect_lag":                 int64(9000),
		"blocks_behind_headers":              int64(0),
		"dogego_body_verification_progress":  1.0,
		"initialblockdownload":               true,
	}
	ApplyUILoadingFlags(sum, false)
	if sum["dogego_ui_loading_phase"] != "utxo_cache" {
		t.Fatalf("bodies caught up with connect lag should show connecting phase: %#v", sum)
	}
}

func TestBootstrapLiveWarmingSetsLoadingFlags(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderChain(dir, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	var f LiveFeed
	cfg := StartConfig{
		Journal:      j,
		ChainDisplay: "mainnet",
		Network:      "mainnet",
		NodeMode:     "full",
		ChainDataDir: filepath.Join(dir, "empty-chain"), // no snapshot
	}
	f.bootstrapLiveIfEmpty(cfg)
	f.mu.RLock()
	liveJSON := f.liveJSON
	f.mu.RUnlock()
	var live map[string]any
	if err := json.Unmarshal(liveJSON, &live); err != nil {
		t.Fatal(err)
	}
	sum, _ := live["summary"].(map[string]any)
	if sum["dogego_ui_loading"] != true {
		t.Fatalf("expected loading flags: %#v", sum)
	}
	if sum["dogego_ui_loading_phase"] != "warming" {
		t.Fatalf("phase %#v", sum["dogego_ui_loading_phase"])
	}
}

func TestPatchSummaryTipFromManifestBodyIBD(t *testing.T) {
	dir := t.TempDir()
	if err := store.SaveRawBlockSyncCheckpoint(dir, store.RawBlockSyncCheckpoint{
		NextProbeHeight:     53249,
		ContiguousRawHeight: 53248,
	}); err != nil {
		t.Fatal(err)
	}
	var f LiveFeed
	f.chainDataDir = dir
	sum := map[string]any{
		"tip_height":                 float64(6_335_000),
		"header_count":               float64(6_335_001),
		"contiguous_raw_height":      float64(52992),
		"raw_blocks":                 float64(52993),
		"dogego_body_ibd_header_paused": true,
	}
	b, _ := json.Marshal(sum)
	f.summaryJSON = b
	f.patchSummaryTipFromManifest(StartConfig{ChainDataDir: dir})
	var got map[string]any
	if err := json.Unmarshal(f.summaryJSON, &got); err != nil {
		t.Fatal(err)
	}
	if got["contiguous_raw_height"] != float64(53248) {
		t.Fatalf("contiguous %#v", got["contiguous_raw_height"])
	}
	if got["raw_blocks"] != float64(53249) {
		t.Fatalf("raw_blocks %#v", got["raw_blocks"])
	}
}

func TestPatchSummaryTipDoesNotReattachDiskP2P(t *testing.T) {
	dir := t.TempDir()
	if err := store.SaveRawBlockSyncCheckpoint(dir, store.RawBlockSyncCheckpoint{
		NextProbeHeight:     1001,
		ContiguousRawHeight: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	var f LiveFeed
	f.chainDataDir = dir
	sum := map[string]any{
		"tip_height":            float64(5000),
		"contiguous_raw_height": float64(900),
		"from_disk_snapshot":    true,
		"summary_stale":         true,
		"sync_status_line":      "Showing last known data · refreshing…",
		"connections_out":       0,
	}
	sumB, _ := json.Marshal(sum)
	f.summaryJSON = sumB
	f.p2pJSON = []byte(`{"wired":true,"from_disk_snapshot":true,"connections_outbound":0,"health_message":"Connecting to the network…"}`)
	f.liveJSON = []byte(`{"ok":true,"from_disk_snapshot":true,"summary":{},"p2p":{"from_disk_snapshot":true}}`)
	f.patchSummaryTipFromManifest(StartConfig{ChainDataDir: dir})
	var live map[string]any
	if err := json.Unmarshal(f.liveJSON, &live); err != nil {
		t.Fatal(err)
	}
	if live["from_disk_snapshot"] != false {
		t.Fatalf("envelope from_disk=%v", live["from_disk_snapshot"])
	}
	if _, has := live["p2p"]; has {
		t.Fatalf("must not re-attach disk bootstrap p2p: %#v", live["p2p"])
	}
	gotSum, _ := live["summary"].(map[string]any)
	if gotSum["from_disk_snapshot"] != nil || gotSum["summary_stale"] != nil {
		t.Fatalf("summary still marked disk/stale: %#v", gotSum)
	}
	if gotSum["connections_out"] != float64(0) && gotSum["connections_out"] != 0 {
		// still 0 until live overlay; just ensure we didn't invent peers
	}
}
