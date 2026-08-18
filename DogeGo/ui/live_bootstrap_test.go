// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func TestMaybeReconcileRawBlockCountRateLimit(t *testing.T) {
	rawCountReconcileAt = time.Time{}
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	// No files on disk; contiguous says bodies through 5 - first call reconciles, still estimates.
	got := maybeReconcileRawBlockCount(raw, 5, 0)
	if got != 6 {
		t.Fatalf("first pass got %d want 6", got)
	}
	rawCountReconcileAt = time.Now()
	got2 := maybeReconcileRawBlockCount(raw, 5, 0)
	if got2 != 6 {
		t.Fatalf("rate-limited estimate got %d want 6", got2)
	}
}

func TestBootstrapLiveIfEmpty(t *testing.T) {
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
	}
	f.bootstrapLiveIfEmpty(cfg)
	f.mu.RLock()
	hasLive := len(f.liveJSON) > 0
	liveJSON := f.liveJSON
	f.mu.RUnlock()
	if !hasLive {
		t.Fatal("expected bootstrap live JSON")
	}
	var live map[string]any
	if err := json.Unmarshal(liveJSON, &live); err != nil {
		t.Fatal(err)
	}
	if live["ok"] != true {
		t.Fatalf("bootstrap live ok=%v", live["ok"])
	}
	sum, _ := live["summary"].(map[string]any)
	if sum == nil || sum["dogego_sync_ok"] != true {
		t.Fatalf("bootstrap summary missing dogego_sync_ok: %#v", sum)
	}
	if sum["dogego_ui_loading"] != true {
		t.Fatalf("expected dogego_ui_loading on warming bootstrap: %#v", sum)
	}
}

func TestLiveFeedSkipsOverlappingRefresh(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	var f LiveFeed
	f.summaryBuild = func(StartConfig) (map[string]any, error) {
		n := calls.Add(1)
		if n == 1 {
			close(started)
			<-release
		}
		return map[string]any{"ok": true, "tip_height": n}, nil
	}
	done := make(chan struct{})
	go func() {
		f.refresh(StartConfig{})
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first summary build did not start")
	}
	f.refresh(StartConfig{})
	if calls.Load() != 1 {
		t.Fatalf("overlapping refresh started a second BuildSummaryMap: calls=%d", calls.Load())
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("first refresh did not finish")
	}
	if calls.Load() != 1 {
		t.Fatalf("first refresh should be the only build, calls=%d", calls.Load())
	}
}

func TestLiveFeedPublishesP2PWhileSummaryBlocked(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var f LiveFeed
	f.summaryBuild = func(StartConfig) (map[string]any, error) {
		close(started)
		<-release
		return map[string]any{"ok": true, "tip_height": float64(1)}, nil
	}
	cfg := StartConfig{
		P2PSnapshot: func() map[string]any {
			return map[string]any{
				"wired":                   true,
				"contiguous_block_height": 55013,
				"ibd_progress": map[string]any{
					"blocks_per_minute":     12.5,
					"in_flight_batches":     6,
					"lowest_missing_height": 55014,
				},
				"dogego_sync_activity": map[string]any{
					"headline": "Downloading block bodies from height 55014",
				},
			}
		},
	}
	done := make(chan struct{})
	go func() {
		f.refresh(cfg)
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("summary build did not start")
	}
	f.refresh(cfg)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		f.mu.RLock()
		live := append([]byte(nil), f.liveJSON...)
		f.mu.RUnlock()
		if len(live) > 0 {
			var m map[string]any
			if json.Unmarshal(live, &m) == nil {
				p2, _ := m["p2p"].(map[string]any)
				sum, _ := m["summary"].(map[string]any)
				if p2["wired"] == true && m["from_disk_snapshot"] == false && sum["blocks_per_minute"] == 12.5 {
					close(release)
					select {
					case <-done:
					case <-time.After(2 * time.Second):
					}
					return
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	close(release)
	t.Fatal("expected live P2P overlay while BuildSummaryMap blocked")
}
