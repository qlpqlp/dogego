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
