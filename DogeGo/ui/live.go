// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dogego/rpc"
	"dogego/store"
)

// LiveFeed serves dashboard API responses from a background refresh loop so sync never blocks HTTP.
type LiveFeed struct {
	mu sync.RWMutex

	summaryJSON []byte
	p2pJSON     []byte
	mempoolJSON []byte
	liveJSON    []byte

	analyticsJSON []byte

	chainStatsJSON []byte
	chainStatsAt   time.Time

	chainDataDir string

	refreshing atomic.Bool
	// summaryBuild, when set, replaces BuildSummaryMap (tests).
	summaryBuild func(StartConfig) (map[string]any, error)
}

// StartLiveFeed runs periodic refresh until ctx is cancelled. interval should be ~500ms-1s during IBD.
func StartLiveFeed(ctx context.Context, cfg StartConfig, interval time.Duration) *LiveFeed {
	f := &LiveFeed{chainDataDir: strings.TrimSpace(cfg.ChainDataDir)}
	if interval <= 0 {
		interval = 750 * time.Millisecond
	}
	f.bootstrapLiveIfEmpty(cfg)
	go f.loop(ctx, cfg, interval)
	return f
}

func (f *LiveFeed) loop(ctx context.Context, cfg StartConfig, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	f.refresh(cfg)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			f.refresh(cfg)
		}
	}
}

func (f *LiveFeed) buildSummary(cfg StartConfig) (map[string]any, error) {
	if f != nil && f.summaryBuild != nil {
		return f.summaryBuild(cfg)
	}
	return BuildSummaryMap(cfg)
}

func (f *LiveFeed) refresh(cfg StartConfig) {
	// One BuildSummaryMap at a time. The old 12s timeout returned while the
	// goroutine kept running, so IBD + a busy dashboard stacked dozens of
	// full-chain summaries until the process died (~15 GB RSS).
	if !f.refreshing.CompareAndSwap(false, true) {
		f.patchSummaryTipFromManifest(cfg)
		return
	}

	done := make(chan struct{})
	go func() {
		defer func() {
			f.refreshing.Store(false)
			close(done)
		}()
		sum, err := f.buildSummary(cfg)
		f.commitRefresh(cfg, sum, err)
	}()
	select {
	case <-done:
	case <-time.After(12 * time.Second):
		f.patchSummaryTipFromManifest(cfg)
		f.bootstrapLiveIfEmpty(cfg)
	}
}

func (f *LiveFeed) commitRefresh(cfg StartConfig, sum map[string]any, sumErr error) {
	var sumB, p2pB, mpB []byte
	if sumErr == nil {
		sumB, _ = json.Marshal(sum)
	} else {
		f.mu.RLock()
		prevSummary := f.summaryJSON
		prevP2P := f.p2pJSON
		prevMempool := f.mempoolJSON
		f.mu.RUnlock()
		if len(prevSummary) > 0 {
			var snap map[string]any
			if json.Unmarshal(prevSummary, &snap) == nil {
				live := map[string]any{
					"ok":            true,
					"summary":       snap,
					"summary_stale": true,
					"summary_error": sumErr.Error(),
				}
				if len(prevP2P) > 0 {
					var p2 map[string]any
					if json.Unmarshal(prevP2P, &p2) == nil {
						live["p2p"] = p2
					}
				}
				if len(prevMempool) > 0 {
					var mp any
					if json.Unmarshal(prevMempool, &mp) == nil {
						live["mempool"] = mp
					}
				}
				liveB, _ := json.Marshal(live)
				f.mu.Lock()
				f.liveJSON = liveB
				f.mu.Unlock()
				return
			}
		}
	}
	if cfg.P2PSnapshot != nil {
		if snap := p2PSnapshotWithTimeout(cfg.P2PSnapshot); snap != nil {
			p2pB, _ = json.Marshal(snap)
		}
	} else {
		p2pB, _ = json.Marshal(map[string]any{"wired": false})
	}
	mpB, _ = json.Marshal(MempoolDetailForAPI(cfg.Pool, 200, cfg.EffectiveFile, cfg.OrphanCount))
	live := map[string]any{"ok": sumErr == nil}
	if sumErr == nil {
		clearSummaryStaleFlags(sum)
		ApplyUILoadingFlags(sum, false)
		live["summary"] = sum
		sumB, _ = json.Marshal(sum)
	} else {
		live["summary_error"] = sumErr.Error()
	}
	if len(p2pB) > 0 {
		var p2 map[string]any
		if json.Unmarshal(p2pB, &p2) == nil {
			live["p2p"] = p2
		}
	}
	if len(mpB) > 0 {
		var mp any
		if json.Unmarshal(mpB, &mp) == nil {
			live["mempool"] = mp
		}
	}
	f.mu.RLock()
	analyticsCopy := append([]byte(nil), f.analyticsJSON...)
	f.mu.RUnlock()
	if len(analyticsCopy) > 0 {
		var an any
		if json.Unmarshal(analyticsCopy, &an) == nil {
			live["analytics_summary"] = an
		}
	}
	liveB, _ := json.Marshal(live)

	f.mu.Lock()
	f.summaryJSON = sumB
	f.p2pJSON = p2pB
	f.mempoolJSON = mpB
	f.liveJSON = liveB
	f.mu.Unlock()

	if sumErr == nil && sum != nil {
		dir := f.chainDataDir
		if dir == "" {
			dir = strings.TrimSpace(cfg.ChainDataDir)
		}
		go f.persistSnapshotAsync(dir, sum, p2pB, mpB, analyticsCopy)
	}
}

// patchSummaryTipFromManifest bumps cached tip/body fields when BuildSummaryMap blocks.
// Header IBD used the segment manifest; body IBD must also read rawblocks_sync.json because
// headers are already at tip so the old early-return left the dashboard frozen.
func (f *LiveFeed) patchSummaryTipFromManifest(cfg StartConfig) {
	var headerTip int64 = -1
	var headerHash string
	if cfg.Journal != nil {
		if m, ok := store.ReadSegmentManifest(cfg.Journal.ChainDir()); ok {
			headerTip = m.TipHeight
			headerHash = m.TipHashHex
		}
	}
	dir := f.chainDataDir
	if dir == "" {
		dir = strings.TrimSpace(cfg.ChainDataDir)
	}
	contig := int64(-1)
	if dir != "" {
		if cp, err := store.LoadRawBlockSyncCheckpoint(dir); err == nil && cp.ContiguousRawHeight >= 0 {
			contig = cp.ContiguousRawHeight
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.summaryJSON) == 0 {
		return
	}
	var snap map[string]any
	if json.Unmarshal(f.summaryJSON, &snap) != nil {
		return
	}
	changed := false
	oldTip, _ := snap["tip_height"].(float64)
	if headerTip >= 0 && float64(headerTip) > oldTip {
		snap["tip_height"] = headerTip
		snap["header_count"] = headerTip + 1
		if headerHash != "" {
			snap["best_hash"] = headerHash
		}
		changed = true
		if peerStart, ok := snap["peer_start_height"].(float64); ok && peerStart > float64(headerTip) {
			bodyPaused, _ := snap["dogego_body_ibd_header_paused"].(bool)
			if !bodyPaused {
				contiguous := float64(-1)
				if v, ok := snap["contiguous_raw_height"].(float64); ok {
					contiguous = v
				}
				bodyPaused = rpc.BodyIBDOwnsPipeline(headerTip, int64(contiguous))
			}
			if !bodyPaused {
				pct := int(float64(headerTip) / peerStart * 100)
				if pct > 100 {
					pct = 100
				}
				line := fmt.Sprintf("Synchronizing headers… %d%% (height %d / ~%d)", pct, headerTip, int64(peerStart))
				snap["sync_status_line"] = line
				snap["dogego_sync_status"] = line
			}
		}
	}
	oldCont, hasCont := snap["contiguous_raw_height"].(float64)
	if contig >= 0 && (!hasCont || float64(contig) > oldCont) {
		snap["contiguous_raw_height"] = contig
		if rb, ok := snap["raw_blocks"].(float64); !ok || rb < float64(contig)+1 {
			snap["raw_blocks"] = contig + 1
		}
		if tip, ok := jsonFloat(snap["tip_height"]); ok && tip > 0 {
			body := (float64(contig) + 1) / (tip + 1)
			snap["dogego_body_verification_progress"] = body
			paused, _ := snap["dogego_body_ibd_header_paused"].(bool)
			if paused || rpc.BodyIBDOwnsPipeline(int64(tip), contig) {
				snap["verification_progress"] = body
				snap["sync_pct"] = body * 100
			}
		}
		changed = true
	}
	if !changed {
		return
	}
	if b, err := json.Marshal(snap); err == nil {
		f.summaryJSON = b
		live := map[string]any{"ok": true, "summary": snap}
		if len(f.p2pJSON) > 0 {
			var p2 map[string]any
			if json.Unmarshal(f.p2pJSON, &p2) == nil {
				live["p2p"] = p2
			}
		}
		if len(f.mempoolJSON) > 0 {
			var mp any
			if json.Unmarshal(f.mempoolJSON, &mp) == nil {
				live["mempool"] = mp
			}
		}
		if liveB, err := json.Marshal(live); err == nil {
			f.liveJSON = liveB
		}
	}
}

func jsonFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

// bootstrapLiveIfEmpty publishes a disk snapshot or minimal warming summary when liveJSON is empty.
func (f *LiveFeed) bootstrapLiveIfEmpty(cfg StartConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.liveJSON) > 0 {
		return
	}
	dir := f.chainDataDir
	if dir == "" {
		dir = strings.TrimSpace(cfg.ChainDataDir)
	}
	if snap, err := loadDashboardSnapshot(dir); err == nil && snap != nil && snap.Summary != nil {
		sum := snap.Summary
		markSummaryFromDiskSnapshot(sum)
		sumB, _ := json.Marshal(sum)
		p2pB := snap.P2P
		if len(p2pB) == 0 {
			p2pB = []byte(`{"wired":false,"from_disk_snapshot":true}`)
		}
		mpB := snap.Mempool
		if len(mpB) == 0 {
			mpB, _ = json.Marshal(MempoolDetailForAPI(cfg.Pool, 200, cfg.EffectiveFile, cfg.OrphanCount))
		}
		live := map[string]any{
			"ok":                 true,
			"summary":            sum,
			"summary_stale":      true,
			"from_disk_snapshot": true,
		}
		var p2 map[string]any
		if json.Unmarshal(p2pB, &p2) == nil {
			live["p2p"] = p2
		}
		var mp any
		if json.Unmarshal(mpB, &mp) == nil {
			live["mempool"] = mp
		}
		if len(snap.AnalyticsSummary) > 0 {
			live["analytics_summary"] = snap.AnalyticsSummary
			if anB, err := json.Marshal(snap.AnalyticsSummary); err == nil {
				f.analyticsJSON = anB
			}
		}
		liveB, _ := json.Marshal(live)
		f.summaryJSON = sumB
		f.p2pJSON = p2pB
		f.mempoolJSON = mpB
		f.liveJSON = liveB
		return
	}
	if cfg.Journal == nil {
		return
	}
	sum := warmingSummaryFromManifest(cfg)
	if sum == nil {
		return
	}
	sumB, _ := json.Marshal(sum)
	mpB, _ := json.Marshal(MempoolDetailForAPI(cfg.Pool, 200, cfg.EffectiveFile, cfg.OrphanCount))
	p2pB := []byte(`{"wired":false,"warming_up":true}`)
	live := map[string]any{"ok": true, "summary": sum, "warming_up": true}
	if len(p2pB) > 0 {
		var p2 map[string]any
		if json.Unmarshal(p2pB, &p2) == nil {
			live["p2p"] = p2
		}
	}
	if len(mpB) > 0 {
		var mp any
		if json.Unmarshal(mpB, &mp) == nil {
			live["mempool"] = mp
		}
	}
	liveB, _ := json.Marshal(live)
	f.summaryJSON = sumB
	f.p2pJSON = p2pB
	f.mempoolJSON = mpB
	f.liveJSON = liveB
}

func warmingSummaryFromManifest(cfg StartConfig) map[string]any {
	tip, cnt, err := journalTipForDashboard(cfg.Journal)
	if err != nil || tip < 0 {
		return nil
	}
	best, _ := journalBestHashForDashboard(cfg.Journal)
	cont := contiguousHeightForAPI(cfg)
	nm := strings.TrimSpace(cfg.NodeMode)
	if nm == "" {
		nm = "full"
	}
	peer := ""
	if cfg.PeerLabel != nil {
		peer = *cfg.PeerLabel
	}
	mp := 0
	if cfg.Pool != nil {
		mp = cfg.Pool.Count()
	}
	sum := map[string]any{
		"chain":                 cfg.ChainDisplay,
		"network":               cfg.Network,
		"node_mode":             nm,
		"peer":                  peer,
		"tip_height":            tip,
		"header_count":          cnt,
		"best_hash":             best,
		"contiguous_raw_height": cont,
		"mempool_txs":           mp,
		"rpc_addr":              cfg.RPCAddr,
		"ibd_active":            true,
		"initialblockdownload":  true,
		"sync_status_line":      "Loading local data…",
		"dogego_sync_status":    "Loading local data…",
		"dogego_sync_health":    "forward_ibd_starting",
		"dogego_sync_ok":        true,
		"wallet_enabled":        walletLoaded(cfg),
		"wallet_rpc_ready":      walletRPCReady(cfg),
		"wallet_address_ready":  walletAddressReady(cfg),
		"wallet_address":        walletAddr(cfg.Wallet),
		"warming_up":            true,
	}
	mergeVersionFields(sum)
	if cont >= 0 && tip > cont {
		sum["blocks_behind_headers"] = tip - cont
	}
	ApplyUILoadingFlags(sum, true)
	return sum
}

func (f *LiveFeed) persistSnapshotAsync(chainDataDir string, sum map[string]any, p2pB, mpB, analyticsB []byte) {
	if chainDataDir == "" || sum == nil {
		return
	}
	snap := &dashboardSnapshotFile{
		TipHeight: tipHeightFromSummary(sum),
		Summary:   cloneSummaryMap(sum),
		P2P:       append([]byte(nil), p2pB...),
		Mempool:   append([]byte(nil), mpB...),
	}
	clearSummaryStaleFlags(snap.Summary)
	clearUILoading(snap.Summary)
	if len(analyticsB) > 0 {
		var an map[string]any
		if json.Unmarshal(analyticsB, &an) == nil {
			snap.AnalyticsSummary = an
		}
	}
	_ = saveDashboardSnapshot(chainDataDir, snap)
}

func cloneSummaryMap(sum map[string]any) map[string]any {
	if sum == nil {
		return nil
	}
	b, err := json.Marshal(sum)
	if err != nil {
		return nil
	}
	var out map[string]any
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

// RememberAnalytics stores a slim analytics summary for /api/live and disk snapshot.
func (f *LiveFeed) RememberAnalytics(detail map[string]any) {
	if f == nil || detail == nil {
		return
	}
	b, err := json.Marshal(detail)
	if err != nil {
		return
	}
	f.mu.Lock()
	f.analyticsJSON = b
	// Refresh liveJSON analytics field if we already have a live payload.
	if len(f.liveJSON) > 0 {
		var live map[string]any
		if json.Unmarshal(f.liveJSON, &live) == nil {
			live["analytics_summary"] = detail
			if liveB, err := json.Marshal(live); err == nil {
				f.liveJSON = liveB
			}
		}
	}
	dir := f.chainDataDir
	sumB := f.summaryJSON
	p2pB := f.p2pJSON
	mpB := f.mempoolJSON
	f.mu.Unlock()
	if dir == "" || len(sumB) == 0 {
		return
	}
	var sum map[string]any
	if json.Unmarshal(sumB, &sum) != nil {
		return
	}
	go f.persistSnapshotAsync(dir, sum, p2pB, mpB, b)
}

func (f *LiveFeed) writeSummary(w http.ResponseWriter) {
	f.mu.RLock()
	b := f.summaryJSON
	f.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if len(b) == 0 {
		_, _ = w.Write([]byte(`{"live":false,"note":"dashboard warming up"}`))
		return
	}
	_, _ = w.Write(b)
}

func (f *LiveFeed) writeP2P(w http.ResponseWriter) {
	f.mu.RLock()
	b := f.p2pJSON
	f.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if len(b) == 0 {
		_, _ = w.Write([]byte(`{"wired":false,"live":false}`))
		return
	}
	_, _ = w.Write(b)
}

func (f *LiveFeed) writeMempool(w http.ResponseWriter) {
	f.mu.RLock()
	b := f.mempoolJSON
	f.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if len(b) == 0 {
		_, _ = w.Write([]byte(`{"txs":[],"live":false}`))
		return
	}
	_, _ = w.Write(b)
}

func (f *LiveFeed) writeLive(w http.ResponseWriter) {
	f.mu.RLock()
	b := f.liveJSON
	f.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if len(b) == 0 {
		_, _ = w.Write([]byte(`{"ok":false,"live":false}`))
		return
	}
	_, _ = w.Write(b)
}

// ChainStatsCached returns light chainstats JSON, refreshing at most every minInterval.
func (f *LiveFeed) ChainStatsCached(cfg StartConfig, light bool, minInterval time.Duration) []byte {
	if cfg.Journal == nil {
		return nil
	}
	now := time.Now()
	f.mu.RLock()
	if now.Sub(f.chainStatsAt) < minInterval && len(f.chainStatsJSON) > 0 {
		b := f.chainStatsJSON
		f.mu.RUnlock()
		return b
	}
	f.mu.RUnlock()

	chainActive, stored := chainStatsHints(cfg)
	stats := BuildChainStats(cfg.Journal, cfg.RawBlocks, cfg.PubkeyHashAddrID, now, chainActive, stored, light)
	b, err := json.Marshal(stats)
	if err != nil {
		return nil
	}
	f.mu.Lock()
	f.chainStatsJSON = b
	f.chainStatsAt = now
	f.mu.Unlock()
	return b
}
