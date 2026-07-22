// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
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
