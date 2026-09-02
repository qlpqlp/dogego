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
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dogego/rpc"
	"dogego/store"
	"dogego/wallet"
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
	p2pBusy       atomic.Bool
	p2pBusySince  atomic.Int64 // unix nano; watchdog when P2PSnapshot blocks
	// HTTP handlers read these without taking f.mu during IBD marshal.
	summaryAtomic atomic.Value // []byte
	liveAtomic    atomic.Value // []byte
	p2pAtomic     atomic.Value // []byte
	lastHeavyAt   atomic.Int64
	// summaryBuild, when set, replaces BuildSummaryMap (tests).
	summaryBuild func(StartConfig) (map[string]any, error)

	// contRate* estimates blk/min from contiguous tip movement when ibd_progress is missing.
	contRateHeight int64
	contRateAt     time.Time
}

const (
	liveOverlayInterval   = 250 * time.Millisecond
	liveHeavyIBDInterval  = 15 * time.Second
	// liveP2POverlayTimeout bounds commitLiveP2P. Unbounded P2PSnapshot blocked forever
	// during IBD lock contention, leaving /api/live on zeroed disk bootstrap (peers=0).
	liveP2POverlayTimeout = 5 * time.Second
	liveP2PBusyWatchdog     = 12 * time.Second
)

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
	overlay := time.NewTicker(liveOverlayInterval)
	defer overlay.Stop()
	f.refresh(cfg)
	f.lastHeavyAt.Store(time.Now().UnixNano())
	for {
		select {
		case <-ctx.Done():
			return
		case <-overlay.C:
			f.publishLiveProgress(cfg)
		case <-tick.C:
			if liveIBDActive(cfg) {
				last := time.Unix(0, f.lastHeavyAt.Load())
				if !last.IsZero() && time.Since(last) < liveHeavyIBDInterval {
					f.publishLiveProgress(cfg)
					continue
				}
			}
			f.refresh(cfg)
			f.lastHeavyAt.Store(time.Now().UnixNano())
		}
	}
}

func liveIBDActive(cfg StartConfig) bool {
	if cfg.ActiveJournal() == nil {
		return false
	}
	tip, _, err := journalTipForDashboard(cfg.ActiveJournal())
	if err != nil || tip < 0 {
		return false
	}
	cont := contiguousHeightForAPI(cfg)
	if cont < 0 {
		return tip > 64
	}
	return tip-cont > 64
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
		f.publishLiveProgress(cfg)
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
		f.publishLiveProgress(cfg)
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
				f.publishCachedJSON(nil, nil, nil, liveB)
				return
			}
		}
	}
	if cfg.P2PSnapshot != nil {
		var snap map[string]any
		if s := p2PSnapshotWithTimeout(cfg.P2PSnapshot); s != nil {
			snap = s
		} else {
			snap = peerCountSnapFromRPC(cfg)
		}
		snap = enrichP2PSnapFromRPC(cfg, snap)
		if snap != nil {
			delete(snap, "from_disk_snapshot")
			storeP2PSnapshotCache(snap)
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
	// Keep fat analytics off the /api/live hot path (see commitLiveP2P).
	liveB, _ := json.Marshal(live)

	f.publishCachedJSON(sumB, p2pB, mpB, liveB)

	if sumErr == nil && sum != nil {
		dir := f.chainDataDir
		if dir == "" {
			dir = strings.TrimSpace(cfg.ChainDataDir)
		}
		f.mu.RLock()
		analyticsCopy := append([]byte(nil), f.analyticsJSON...)
		f.mu.RUnlock()
		go f.persistSnapshotAsync(dir, sum, p2pB, mpB, analyticsCopy)
	}
}

// publishLiveProgress updates the dashboard from the body checkpoint and a live P2P
// snapshot without waiting for BuildSummaryMap (which can block for minutes on perfile IBD).
func (f *LiveFeed) publishLiveProgress(cfg StartConfig) {
	f.patchSummaryTipFromManifest(cfg)
	f.bootstrapLiveIfEmpty(cfg)
	if !f.tryBeginP2POverlay() {
		return
	}
	go func() {
		defer f.endP2POverlay()
		f.commitLiveP2P(cfg)
	}()
}

func (f *LiveFeed) tryBeginP2POverlay() bool {
	if f.p2pBusy.CompareAndSwap(false, true) {
		f.p2pBusySince.Store(time.Now().UnixNano())
		return true
	}
	since := f.p2pBusySince.Load()
	if since > 0 && time.Since(time.Unix(0, since)) > liveP2PBusyWatchdog {
		// Prior overlay goroutine hung inside P2PSnapshot — recover so live metrics resume.
		f.p2pBusy.Store(false)
		if f.p2pBusy.CompareAndSwap(false, true) {
			f.p2pBusySince.Store(time.Now().UnixNano())
			return true
		}
	}
	return false
}

func (f *LiveFeed) endP2POverlay() {
	f.p2pBusy.Store(false)
	f.p2pBusySince.Store(0)
}

func (f *LiveFeed) commitLiveP2P(cfg StartConfig) {
	var snap map[string]any
	if cfg.P2PSnapshot != nil {
		snap = p2PSnapshotWithTimeoutDur(cfg.P2PSnapshot, liveP2POverlayTimeout)
		if snap == nil {
			// Prefer last full snap (with ibd_progress) over a peer-count-only RPC stub.
			if cached := cachedP2PSnapshot(2 * time.Minute); cached != nil && cached["from_disk_snapshot"] != true {
				snap = cached
			}
		}
	}
	if snap == nil {
		// Full P2P snapshot can block on IBD locks; still publish peer counts from getpeerinfo
		// so the dock is not stuck on cold-start zeros while /api/peers already shows links.
		snap = peerCountSnapFromRPC(cfg)
	}
	if snap == nil {
		snap = minimalDialingP2PSnap()
	}
	snap = enrichP2PSnapFromRPC(cfg, snap)
	if snap != nil {
		delete(snap, "from_disk_snapshot")
		storeP2PSnapshotCache(snap)
	}
	p2pB, _ := json.Marshal(snap)
	if snap == nil {
		p2pB = nil
	}
	mpB, _ := json.Marshal(MempoolDetailForAPI(cfg.Pool, 200, cfg.EffectiveFile, cfg.OrphanCount))

	f.mu.RLock()
	prevSum := append([]byte(nil), f.summaryJSON...)
	f.mu.RUnlock()

	var sum map[string]any
	if len(prevSum) > 0 {
		_ = json.Unmarshal(prevSum, &sum)
	}
	if sum == nil {
		sum = map[string]any{}
	}
	if cont := contiguousHeightForAPI(cfg); cont >= 0 {
		sum["contiguous_raw_height"] = cont
		sum["dogego_contiguous_raw_height"] = cont
		f.noteContiguousRate(sum, cont)
	}
	var p2 map[string]any
	liveProgress := false
	if len(p2pB) > 0 && json.Unmarshal(p2pB, &p2) == nil {
		overlayP2PProgressOnSummary(sum, p2)
		liveProgress = p2PHasLiveProgress(p2) || p2PHasLiveConnections(p2)
	}
	if cfg.StorageSummary != nil {
		if st := cfg.StorageSummary(); st != nil {
			mergeStorageSummary(sum, st)
		}
	}
	// Full BuildSummaryMap may still be deferred during IBD, but contiguous height +
	// ibd_progress overlays are live — clear disk-snapshot flags so the sync dock
	// stops showing "Updating… / last known data" while rates advance.
	if liveProgress || contHeightFromSummary(sum) >= 0 {
		clearSummaryStaleFlags(sum)
		if line, _ := sum["sync_status_line"].(string); strings.Contains(line, "last known data") {
			if h, _ := sum["dogego_sync_status"].(string); h != "" {
				sum["sync_status_line"] = h
			} else if act, _ := sum["dogego_sync_activity"].(map[string]any); act != nil {
				if h, _ := act["headline"].(string); h != "" {
					sum["sync_status_line"] = h
					sum["dogego_sync_status"] = h
				} else {
					delete(sum, "sync_status_line")
				}
			} else {
				delete(sum, "sync_status_line")
			}
		}
	}
	warming := sum["warming_up"] == true
	ApplyUILoadingFlags(sum, warming)
	if sum["dogego_ui_loading_phase"] == "body_reconcile" && !summaryHasLiveSyncWork(sum) {
		sum["warming_up"] = true
	}
	sumB, _ := json.Marshal(sum)
	live := map[string]any{
		"ok":                 true,
		"summary":            sum,
		"from_disk_snapshot": false,
	}
	if sum["warming_up"] == true {
		live["warming_up"] = true
	}
	// Envelope summary_stale means "full summary build pending", not frozen IBD metrics.
	if !liveProgress {
		live["summary_stale"] = true
	}
	if p2 != nil {
		live["p2p"] = p2
	} else if len(p2pB) == 0 {
		// Drop stale disk-bootstrap p2p from the envelope when the live snap timed out.
		delete(live, "p2p")
	}
	if len(mpB) > 0 {
		var mp any
		if json.Unmarshal(mpB, &mp) == nil {
			live["mempool"] = mp
		}
	}
	// Do not embed analytics_summary here — full Analytics payloads are 100KB–1MB and
	// re-parsing them on every 250–400ms /api/live poll freezes the dashboard (logs stall too).
	liveB, _ := json.Marshal(live)
	f.publishCachedJSON(sumB, p2pB, mpB, liveB)
}

func asJSONBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return append([]byte(nil), b...)
}

// minimalDialingP2PSnap is the fallback when full P2PSnapshot/RPC are unavailable during startup.
func minimalDialingP2PSnap() map[string]any {
	return map[string]any{
		"wired":                true,
		"peer_dialing":         true,
		"warming_up":           true,
		"connections_outbound": 0,
		"connections_inbound":  0,
		"connections_total":    0,
		"health":               "starting",
		"health_message":       "Connecting to the network…",
		"dogego_sync_activity": map[string]any{
			"headline": "Connecting to peers",
			"detail":   "Dialing DNS seeds and addrbook — block and header sync start after the first handshakes.",
		},
	}
}

// peerCountSnapFromRPC builds a minimal live P2P snap from getpeerinfo when the full
// node P2PSnapshot is blocked. Keeps Overview/dock peer counts moving during IBD.
func peerCountSnapFromRPC(cfg StartConfig) map[string]any {
	if cfg.RPCInvoke == nil {
		return minimalDialingP2PSnap()
	}
	rpcOut, timedOut := invokeRPCWithTimeout(cfg.RPCInvoke, "getpeerinfo", nil, 1500*time.Millisecond)
	if timedOut || rpcOut == nil {
		return minimalDialingP2PSnap()
	}
	if errObj, ok := rpcOut["error"].(map[string]interface{}); ok && errObj != nil {
		return minimalDialingP2PSnap()
	}
	res := rpcOut["result"]
	if res == nil {
		return minimalDialingP2PSnap()
	}
	peers := normalizePeerInfoResult(res)
	inN, outN := peerDirectionCounts(peers)
	if inN+outN <= 0 {
		return minimalDialingP2PSnap()
	}
	return map[string]any{
		"wired":                true,
		"peer_dialing":         false,
		"connections_outbound": outN,
		"connections_inbound":  inN,
		"connections_total":    inN + outN,
		"health":               "ok",
		"health_message":       fmt.Sprintf("P2P active with %d outbound sync connection(s).", outN),
	}
}

// peerCountsFromGetPeerInfo returns live inbound/outbound counts from getpeerinfo.
func peerCountsFromGetPeerInfo(cfg StartConfig) (inbound, outbound int, ok bool) {
	if cfg.RPCInvoke == nil {
		return 0, 0, false
	}
	rpcOut, timedOut := invokeRPCWithTimeout(cfg.RPCInvoke, "getpeerinfo", nil, 1500*time.Millisecond)
	if timedOut || rpcOut == nil {
		return 0, 0, false
	}
	if errObj, okErr := rpcOut["error"].(map[string]interface{}); okErr && errObj != nil {
		return 0, 0, false
	}
	res := rpcOut["result"]
	if res == nil {
		return 0, 0, false
	}
	inN, outN := peerDirectionCounts(normalizePeerInfoResult(res))
	if inN+outN <= 0 {
		return 0, 0, false
	}
	return inN, outN, true
}

func p2pSnapConnectionCounts(snap map[string]any) (inbound, outbound, total int) {
	if snap == nil {
		return 0, 0, 0
	}
	inbound, _ = p2pCountField(snap["connections_inbound"])
	outbound, _ = p2pCountField(snap["connections_outbound"])
	total, haveTotal := p2pCountField(snap["connections_total"])
	if !haveTotal || total <= 0 {
		total = inbound + outbound
	}
	if outbound > 0 || total > 0 {
		return inbound, outbound, total
	}
	assist, _ := p2pCountField(snap["block_assist_connections"])
	hdr, _ := p2pCountField(snap["dedicated_header_connections"])
	relay, _ := p2pCountField(snap["connections_outbound_relay"])
	rebuilt := assist + hdr + relay
	if primary, _ := snap["primary_peer"].(string); primary != "" && !strings.HasPrefix(strings.TrimSpace(primary), "(") {
		rebuilt++
	}
	if rebuilt > outbound {
		outbound = rebuilt
		if total < outbound+inbound {
			total = outbound + inbound
		}
	}
	return inbound, outbound, total
}

func p2pCountField(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

// enrichP2PSnapFromRPC merges getpeerinfo counts when the node P2PSnapshot under-reports
// connections during block-assist IBD (snapshot returns peer_dialing/0 while RPC lists peers).
func enrichP2PSnapFromRPC(cfg StartConfig, snap map[string]any) map[string]any {
	if snap == nil {
		snap = peerCountSnapFromRPC(cfg)
	}
	if snap == nil {
		return nil
	}
	healP2PSnapConnectionCounts(snap)
	inSnap, outSnap, totalSnap := p2pSnapConnectionCounts(snap)
	out := snap
	changed := false

	// Only hit getpeerinfo when the snap still looks empty — calling it on every
	// /api/live|/api/p2p request under IBD stalls the HTTP handlers and freezes the UI.
	needRPC := outSnap == 0 && totalSnap == 0
	if needRPC {
		inRPC, outRPC, rpcOK := peerCountsFromGetPeerInfo(cfg)
		if rpcOK && (outRPC > outSnap || inRPC > inSnap) {
			out = cloneStringAnyMap(snap)
			changed = true
			if outRPC > outSnap {
				out["connections_outbound"] = outRPC
				outSnap = outRPC
			}
			if inRPC > inSnap {
				out["connections_inbound"] = inRPC
				inSnap = inRPC
			}
			outTotal := inRPC + outRPC
			if cur, have := p2pCountField(out["connections_total"]); !have || cur < outTotal {
				out["connections_total"] = outTotal
			}
		}
	}
	if outSnap > 0 || inSnap > 0 {
		if !changed {
			out = cloneStringAnyMap(snap)
			changed = true
		}
		out["peer_dialing"] = false
		delete(out, "warming_up")
		delete(out, "from_disk_snapshot")
		if h, _ := out["health"].(string); h == "" || h == "starting" {
			out["health"] = "ok"
			out["health_message"] = fmt.Sprintf("P2P active with %d outbound sync connection(s).", outSnap)
		}
		// RPC/dialing stubs leave "Connecting to peers" forever; replace once peers are live.
		if act, _ := out["dogego_sync_activity"].(map[string]any); act != nil {
			if h, _ := act["headline"].(string); h == "Connecting to peers" || h == "" {
				out["dogego_sync_activity"] = map[string]any{
					"headline": "Syncing blocks",
					"detail":   fmt.Sprintf("%d peer(s) connected — downloading and verifying block bodies.", outSnap),
				}
			}
		} else {
			out["dogego_sync_activity"] = map[string]any{
				"headline": "Syncing blocks",
				"detail":   fmt.Sprintf("%d peer(s) connected — downloading and verifying block bodies.", outSnap),
			}
		}
	}
	// Preserve IBD rates when this snap is a peer-count-only enrich/fallback.
	if out["ibd_progress"] == nil {
		if cached := cachedP2PSnapshot(2 * time.Minute); cached != nil {
			if prog := cached["ibd_progress"]; prog != nil {
				if !changed {
					out = cloneStringAnyMap(out)
					changed = true
				}
				out["ibd_progress"] = prog
			}
			for _, k := range []string{"contiguous_block_height", "ibd_sync_lanes", "block_assist_active", "block_assist_connections"} {
				if out[k] == nil && cached[k] != nil {
					if !changed {
						out = cloneStringAnyMap(out)
						changed = true
					}
					out[k] = cached[k]
				}
			}
		}
	}
	if !changed {
		return snap
	}
	return out
}

func storeJSONAtomic(dst *atomic.Value, b []byte) {
	if dst == nil || len(b) == 0 {
		return
	}
	dst.Store(asJSONBytes(b))
}

func (f *LiveFeed) publishCachedJSON(sumB, p2pB, mpB, liveB []byte) {
	sumB = asJSONBytes(sumB)
	p2pB = asJSONBytes(p2pB)
	mpB = asJSONBytes(mpB)
	liveB = asJSONBytes(liveB)
	f.mu.Lock()
	if len(sumB) > 0 {
		f.summaryJSON = sumB
	}
	if len(p2pB) > 0 {
		f.p2pJSON = p2pB
	}
	if len(mpB) > 0 {
		f.mempoolJSON = mpB
	}
	if len(liveB) > 0 {
		f.liveJSON = liveB
	}
	f.mu.Unlock()
	storeJSONAtomic(&f.summaryAtomic, sumB)
	storeJSONAtomic(&f.p2pAtomic, p2pB)
	storeJSONAtomic(&f.liveAtomic, liveB)
}

func p2PHasLiveProgress(p2p map[string]any) bool {
	if p2p == nil {
		return false
	}
	fromDisk := p2p["from_disk_snapshot"] == true
	if fromDisk {
		if p2p["peer_dialing"] == true || p2p["warming_up"] == true {
			return true
		}
		if act, ok := p2p["dogego_sync_activity"]; ok && act != nil {
			return true
		}
		return p2PHasLiveConnections(p2p)
	}
	if p2p["peer_dialing"] == true || p2p["warming_up"] == true {
		return true
	}
	if prog, _ := p2p["ibd_progress"].(map[string]any); prog != nil {
		return true
	}
	if _, ok := p2p["ibd_progress"].(map[string]interface{}); ok {
		return true
	}
	if act, ok := p2p["dogego_sync_activity"]; ok && act != nil {
		return true
	}
	if _, ok := p2p["contiguous_block_height"]; ok {
		return true
	}
	return p2PHasLiveConnections(p2p)
}

// p2PHasLiveConnections is true when outbound/assist counts show an active sync mesh.
func p2PHasLiveConnections(p2p map[string]any) bool {
	if p2p == nil || p2p["from_disk_snapshot"] == true {
		return false
	}
	toInt := func(v any) (int, bool) {
		switch x := v.(type) {
		case int:
			return x, true
		case int32:
			return int(x), true
		case int64:
			return int(x), true
		case float64:
			return int(x), true
		default:
			return 0, false
		}
	}
	if n, ok := toInt(p2p["connections_outbound"]); ok && n > 0 {
		return true
	}
	if n, ok := toInt(p2p["block_assist_connections"]); ok && n > 0 {
		return true
	}
	if n, ok := toInt(p2p["connections_total"]); ok && n > 0 {
		return true
	}
	return false
}

func contHeightFromSummary(sum map[string]any) int64 {
	if sum == nil {
		return -1
	}
	for _, key := range []string{"contiguous_raw_height", "dogego_contiguous_raw_height"} {
		switch v := sum[key].(type) {
		case int64:
			if v >= 0 {
				return v
			}
		case int:
			if v >= 0 {
				return int64(v)
			}
		case float64:
			if v >= 0 {
				return int64(v)
			}
		}
	}
	return -1
}

func overlayP2PProgressOnSummary(sum, p2p map[string]any) {
	if sum == nil || p2p == nil {
		return
	}
	// Cold disk bootstrap must not paint last-session peers/rates or clear "stale".
	if p2p["from_disk_snapshot"] == true {
		sum["connections_out"] = 0
		sum["connections_in"] = 0
		sum["connections"] = 0
		if p2p["peer_dialing"] == true || p2p["dogego_sync_activity"] != nil {
			clearSummaryStaleFlags(sum)
			if act, ok := p2p["dogego_sync_activity"]; ok && act != nil {
				sum["dogego_sync_activity"] = act
				if m, ok := act.(map[string]any); ok {
					if h, _ := m["headline"].(string); h != "" {
						sum["dogego_sync_status"] = h
						sum["sync_status_line"] = h
					}
				}
			}
		}
		return
	}
	// Live P2P overlay replaces disk-snapshot paint even when BuildSummaryMap is deferred.
	if p2PHasLiveProgress(p2p) {
		clearSummaryStaleFlags(sum)
	}
	// Connection counts live on the P2P snapshot and must refresh even when
	// BuildSummaryMap is deferred during IBD (otherwise Overview/Analytics stay at 0
	// peers while the node is actively dialing).
	overlayP2PConnectionCounts(sum, p2p)
	if act, ok := p2p["dogego_sync_activity"]; ok && act != nil {
		sum["dogego_sync_activity"] = act
		if m, ok := act.(map[string]any); ok {
			if h, _ := m["headline"].(string); h != "" {
				sum["dogego_sync_status"] = h
				sum["sync_status_line"] = h
			}
		}
	}
	if h, _ := p2p["contiguous_block_height"].(float64); h >= 0 {
		sum["contiguous_raw_height"] = h
		sum["dogego_contiguous_raw_height"] = h
	} else if h, ok := p2p["contiguous_block_height"].(int64); ok && h >= 0 {
		sum["contiguous_raw_height"] = h
		sum["dogego_contiguous_raw_height"] = h
	} else if h, ok := p2p["contiguous_block_height"].(int); ok && h >= 0 {
		sum["contiguous_raw_height"] = h
		sum["dogego_contiguous_raw_height"] = h
	}
	prog, _ := p2p["ibd_progress"].(map[string]any)
	if prog == nil {
		if p, ok := p2p["ibd_progress"].(map[string]interface{}); ok {
			prog = p
		}
	}
	if prog == nil {
		return
	}
	if v, ok := prog["blocks_per_minute"]; ok {
		sum["blocks_per_minute"] = v
	}
	if v, ok := prog["headers_per_minute"]; ok {
		sum["headers_per_minute"] = v
		sum["dogego_headers_per_minute"] = v
	}
	if v, ok := prog["contiguous_blocks_per_minute"]; ok {
		sum["contiguous_blocks_per_minute"] = v
		sum["dogego_contiguous_blocks_per_minute"] = v
	}
	if v, ok := prog["frontier_hole_height"]; ok {
		sum["frontier_hole_height"] = v
		sum["dogego_frontier_hole_height"] = v
	}
	if v, ok := prog["raw_blocks_in_flight_ahead"]; ok {
		sum["raw_blocks_in_flight_ahead"] = v
		sum["dogego_raw_blocks_in_flight_ahead"] = v
	}
	if v, ok := prog["hole_blocked_sec"]; ok {
		sum["hole_blocked_sec"] = v
		sum["dogego_hole_blocked_sec"] = v
	}
	if v, ok := prog["in_flight_batches"]; ok {
		sum["in_flight_batches"] = v
	}
	if v, ok := prog["lowest_missing_height"]; ok {
		sum["lowest_missing_height"] = v
	}
	if v, ok := prog["contiguous_raw_height"]; ok {
		sum["contiguous_raw_height"] = v
		sum["dogego_contiguous_raw_height"] = v
	}
	if v, ok := prog["max_blocks_in_transit_per_peer"]; ok {
		sum["dogego_max_blocks_in_transit_per_peer"] = v
	}
	if v, ok := prog["block_stalling_timeout_body_ibd_sec"]; ok {
		sum["dogego_block_stalling_timeout_sec"] = v
	}
	if v, ok := prog["lane_in_flight"]; ok {
		sum["dogego_lane_in_flight"] = v
	}
	if v, ok := prog["sync_workers"]; ok {
		sum["sync_workers"] = v
	}
	if v, ok := prog["blocks_stored_ibd"]; ok {
		sum["blocks_stored_ibd"] = v
	}
	// Recompute sync_eta during lightweight overlays so it doesn't stay stuck
	// on stale values while blocks_per_minute updates frequently during IBD.
	//
	// BuildSummaryMap is intentionally throttled because it can block on heavy
	// I/O (raw header/body sync), so sync_eta may otherwise lag behind the
	// current download rate for tens of seconds.
	toInt64 := func(v any) (int64, bool) {
		switch x := v.(type) {
		case int64:
			return x, true
		case int:
			return int64(x), true
		case float64:
			return int64(x), true
		default:
			return 0, false
		}
	}
	toFloat64 := func(v any) (float64, bool) {
		switch x := v.(type) {
		case float64:
			return x, true
		case int:
			return float64(x), true
		case int64:
			return float64(x), true
		default:
			return 0, false
		}
	}
	tip, okTip := toInt64(sum["tip_height"])
	chainActive, okCA := toInt64(sum["chain_active_height"])
	contiguousH, okCont := toInt64(sum["contiguous_raw_height"])
	// Prefer hole-fill rate for ETA: staged/ingest BPM can spike to thousands while
	// contiguous coverage crawls, which made ETA look like ~20k blk/min that never landed.
	rate, okRate := toFloat64(sum["contiguous_blocks_per_minute"])
	if !okRate || rate <= 0 {
		rate, okRate = toFloat64(sum["blocks_per_minute"])
	}
	if okTip && okCA && okCont && okRate {
		if !math.IsNaN(rate) && !math.IsInf(rate, 0) && rate > 0 {
			behind := rpc.BlocksBehindHeaders(tip, chainActive, contiguousH)
			sum["blocks_behind_headers"] = behind
			sum["sync_eta"] = rpc.FormatSyncETA(behind, rate)
		}
	}
}

func overlayP2PConnectionCounts(sum, p2p map[string]any) {
	if sum == nil || p2p == nil {
		return
	}
	toInt := func(v any) (int, bool) {
		switch x := v.(type) {
		case int:
			return x, true
		case int32:
			return int(x), true
		case int64:
			return int(x), true
		case float64:
			return int(x), true
		default:
			return 0, false
		}
	}
	outN, haveOut := toInt(p2p["connections_outbound"])
	inN, haveIn := toInt(p2p["connections_inbound"])
	// Never invent peer counts from a cold disk snapshot — last-session assist rows
	// would paint "N peers connected" before any dial succeeds.
	if p2p["from_disk_snapshot"] == true {
		sum["connections_out"] = 0
		sum["connections_in"] = 0
		return
	}
	// Recover counts when a live snap briefly has connections_*=0 but assist is up.
	if !haveOut || outN == 0 {
		assist, _ := toInt(p2p["block_assist_connections"])
		hdr, _ := toInt(p2p["dedicated_header_connections"])
		relay, _ := toInt(p2p["connections_outbound_relay"])
		rebuilt := assist + hdr + relay
		if primary, _ := p2p["primary_peer"].(string); primary != "" && !strings.HasPrefix(strings.TrimSpace(primary), "(") {
			rebuilt++
		}
		if rebuilt > 0 {
			outN = rebuilt
			haveOut = true
			p2p["connections_outbound"] = outN
			if total, ok := toInt(p2p["connections_total"]); !ok || total < outN {
				p2p["connections_total"] = outN + inN
			}
		}
	}
	if haveOut {
		sum["connections_out"] = outN
	}
	if haveIn {
		sum["connections_in"] = inN
	}
	if total, ok := toInt(p2p["connections_total"]); ok {
		if !haveOut && !haveIn {
			sum["connections_out"] = total
			sum["connections_in"] = 0
		}
	}
	if v, ok := p2p["p2p_connectivity"].(string); ok && v != "" {
		sum["p2p_connectivity"] = v
	}
	if v, ok := p2p["health"].(string); ok && v != "" {
		sum["p2p_health"] = v
	}
	if v, ok := p2p["health_message"].(string); ok {
		sum["relay_note"] = v
	}
	if v, ok := p2p["inbound_hint"].(string); ok {
		sum["inbound_hint"] = v
	}
	if v, ok := p2p["initialblockdownload"].(bool); ok {
		sum["initialblockdownload"] = v
	}
}

// patchSummaryTipFromManifest bumps cached tip/body fields when BuildSummaryMap blocks.
// Header IBD used the segment manifest; body IBD must also read rawblocks_sync.json because
// headers are already at tip so the old early-return left the dashboard frozen.
func (f *LiveFeed) patchSummaryTipFromManifest(cfg StartConfig) {
	var headerTip int64 = -1
	var headerHash string
	if cfg.ActiveJournal() != nil {
		if m, ok := store.ReadSegmentManifest(cfg.ActiveJournal().ChainDir()); ok {
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
	// Never re-attach a cold disk-bootstrap P2P blob — that permanently paints peers=0
	// over live overlays whenever contiguous/tip advances.
	var liveP2 map[string]any
	if len(f.p2pJSON) > 0 {
		var p2 map[string]any
		if json.Unmarshal(f.p2pJSON, &p2) == nil && p2["from_disk_snapshot"] != true {
			liveP2 = p2
			if p2PHasLiveProgress(p2) || p2PHasLiveConnections(p2) {
				overlayP2PProgressOnSummary(snap, p2)
				clearSummaryStaleFlags(snap)
			}
		}
	}
	// Tip/contig movement is live chain data — drop "last known data" even before P2P lands.
	if contig >= 0 || headerTip >= 0 {
		clearSummaryStaleFlags(snap)
		if line, _ := snap["sync_status_line"].(string); strings.Contains(line, "last known data") {
			delete(snap, "sync_status_line")
			if _, ok := snap["dogego_sync_status"].(string); ok {
				delete(snap, "dogego_sync_status")
			}
		}
	}
	if b, err := json.Marshal(snap); err == nil {
		f.summaryJSON = b
		storeJSONAtomic(&f.summaryAtomic, b)
		live := map[string]any{"ok": true, "summary": snap, "from_disk_snapshot": false}
		if liveP2 != nil {
			live["p2p"] = liveP2
			if p2PHasLiveProgress(liveP2) || p2PHasLiveConnections(liveP2) {
				live["summary"] = snap
			} else {
				live["summary_stale"] = true
			}
		} else {
			live["summary_stale"] = true
		}
		if len(f.mempoolJSON) > 0 {
			var mp any
			if json.Unmarshal(f.mempoolJSON, &mp) == nil {
				live["mempool"] = mp
			}
		}
		if liveB, err := json.Marshal(live); err == nil {
			f.liveJSON = liveB
			storeJSONAtomic(&f.liveAtomic, liveB)
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
			zeroP2PLiveMetrics(p2)
			if pb, err := json.Marshal(p2); err == nil {
				p2pB = pb
			}
			live["p2p"] = p2
		}
		var mp any
		if json.Unmarshal(mpB, &mp) == nil {
			live["mempool"] = mp
		}
		if len(snap.AnalyticsSummary) > 0 {
			// Keep for disk snapshot restore of Analytics panel only — not on /api/live.
			if anB, err := json.Marshal(snap.AnalyticsSummary); err == nil {
				f.analyticsJSON = anB
			}
		}
		liveB, _ := json.Marshal(live)
		f.summaryJSON = asJSONBytes(sumB)
		f.p2pJSON = asJSONBytes(p2pB)
		f.mempoolJSON = asJSONBytes(mpB)
		f.liveJSON = asJSONBytes(liveB)
		storeJSONAtomic(&f.summaryAtomic, sumB)
		storeJSONAtomic(&f.p2pAtomic, p2pB)
		storeJSONAtomic(&f.liveAtomic, liveB)
		return
	}
	if cfg.ActiveJournal() == nil {
		return
	}
	sum := warmingSummaryFromManifest(cfg)
	if sum == nil {
		return
	}
	sumB, _ := json.Marshal(sum)
	mpB, _ := json.Marshal(MempoolDetailForAPI(cfg.Pool, 200, cfg.EffectiveFile, cfg.OrphanCount))
	p2pB := []byte(`{"wired":true,"peer_dialing":true,"warming_up":true}`)
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
	f.summaryJSON = asJSONBytes(sumB)
	f.p2pJSON = asJSONBytes(p2pB)
	f.mempoolJSON = asJSONBytes(mpB)
	f.liveJSON = asJSONBytes(liveB)
	storeJSONAtomic(&f.summaryAtomic, sumB)
	storeJSONAtomic(&f.p2pAtomic, p2pB)
	storeJSONAtomic(&f.liveAtomic, liveB)
}

func warmingSummaryFromManifest(cfg StartConfig) map[string]any {
	tip, cnt, err := journalTipForDashboard(cfg.ActiveJournal())
	if err != nil || tip < 0 {
		return nil
	}
	best, _ := journalBestHashForDashboard(cfg.ActiveJournal())
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
		"wallet_address":        walletAddr(cfg.ActiveWallet()),
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

// PatchWalletEncryptionStatus updates cached /api/summary wallet lock fields immediately
// after walletpassphrase / walletlock so the dashboard does not keep showing "Locked"
// until the next full BuildSummaryMap (often deferred during IBD).
func (f *LiveFeed) PatchWalletEncryptionStatus(w *wallet.Disk) {
	if f == nil || w == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.summaryJSON) == 0 {
		return
	}
	var sum map[string]any
	if json.Unmarshal(f.summaryJSON, &sum) != nil || sum == nil {
		return
	}
	attachWalletEncryptionStatus(sum, w)
	if !w.IsEncrypted() || !w.IsUnlocked() {
		delete(sum, "wallet_unlocked_until")
	}
	sumB, err := json.Marshal(sum)
	if err != nil {
		return
	}
	f.summaryJSON = sumB
	storeJSONAtomic(&f.summaryAtomic, sumB)
	if len(f.liveJSON) == 0 {
		return
	}
	var live map[string]any
	if json.Unmarshal(f.liveJSON, &live) != nil || live == nil {
		return
	}
	live["summary"] = sum
	liveB, err := json.Marshal(live)
	if err != nil {
		return
	}
	f.liveJSON = liveB
	storeJSONAtomic(&f.liveAtomic, liveB)
}

// RememberAnalytics stores analytics for disk snapshot / dedicated analytics routes.
// It must NOT embed the full payload into /api/live (that froze the dashboard during IBD).
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
	// Strip any previously embedded analytics_summary from the live poll envelope.
	if len(f.liveJSON) > 0 {
		var live map[string]any
		if json.Unmarshal(f.liveJSON, &live) == nil && live["analytics_summary"] != nil {
			delete(live, "analytics_summary")
			if liveB, err := json.Marshal(live); err == nil {
				f.liveJSON = liveB
				storeJSONAtomic(&f.liveAtomic, liveB)
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

func (f *LiveFeed) cachedJSON(atom *atomic.Value, fallback []byte) []byte {
	if atom != nil {
		if v := atom.Load(); v != nil {
			if b, ok := v.([]byte); ok && len(b) > 0 {
				return b
			}
		}
	}
	return fallback
}

func (f *LiveFeed) writeSummary(w http.ResponseWriter) {
	f.mu.RLock()
	b := f.cachedJSON(&f.summaryAtomic, f.summaryJSON)
	f.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if len(b) == 0 {
		_, _ = w.Write([]byte(`{"live":true,"note":"dashboard warming up"}`))
		return
	}
	_, _ = w.Write(b)
}

func (f *LiveFeed) writeP2P(w http.ResponseWriter, cfg StartConfig) {
	f.mu.RLock()
	b := f.cachedJSON(&f.p2pAtomic, f.p2pJSON)
	f.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if len(b) == 0 {
		// Request path must stay non-blocking — never call getpeerinfo here.
		_, _ = w.Write([]byte(`{"wired":true,"peer_dialing":true,"live":true,"health":"starting"}`))
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

func (f *LiveFeed) writeLive(w http.ResponseWriter, cfg StartConfig) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var b []byte
	if v := f.liveAtomic.Load(); v != nil {
		if raw, ok := v.([]byte); ok && len(raw) > 0 {
			b = raw
		}
	}
	if len(b) == 0 {
		f.mu.RLock()
		b = f.liveJSON
		f.mu.RUnlock()
	}
	if len(b) == 0 {
		_, _ = w.Write([]byte(`{"ok":true,"live":true,"summary":{"dogego_ui_loading":true,"dogego_sync_ok":true},"warming_up":true}`))
		return
	}
	// Hot path: serve precomputed JSON. Only patch when the envelope is missing p2p or
	// still marked as a disk snapshot — never call getpeerinfo / re-marshal analytics here.
	if bytesContainsAny(b, []string{`"p2p":`, `"from_disk_snapshot":true`, `"summary_stale":true`}) {
		if patched := injectLiveP2PFromCache(cfg, b, f); len(patched) > 0 {
			b = patched
		}
		if fixed := sanitizeLiveJSONIfP2PActive(b); len(fixed) > 0 {
			b = fixed
		}
	}
	if fixed := overlayContiguousReconcileOnLiveJSON(b); len(fixed) > 0 {
		b = fixed
	}
	_, _ = w.Write(b)
}

func bytesContainsAny(b []byte, needles []string) bool {
	s := string(b)
	// Fast path: if p2p is absent we must inject; if disk/stale flags present we may sanitize.
	if !strings.Contains(s, `"p2p"`) {
		return true
	}
	for _, n := range needles[1:] {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// injectLiveP2PFromCache attaches or refreshes live.p2p from the P2P cache when the envelope
// dropped it (slow summary refresh). Cache-only — no RPC (request path must not block).
func injectLiveP2PFromCache(cfg StartConfig, liveB []byte, f *LiveFeed) []byte {
	var live map[string]any
	if json.Unmarshal(liveB, &live) != nil || live == nil {
		return nil
	}
	f.mu.RLock()
	p2b := f.cachedJSON(&f.p2pAtomic, f.p2pJSON)
	sumB := f.cachedJSON(&f.summaryAtomic, f.summaryJSON)
	f.mu.RUnlock()
	var p2 map[string]any
	if len(p2b) == 0 || json.Unmarshal(p2b, &p2) != nil || p2 == nil {
		return nil
	}
	// Soft heal without RPC (assist/primary rebuild + clear dialing stub text).
	healP2PSnapConnectionCounts(p2)
	_, outSnap, totalSnap := p2pSnapConnectionCounts(p2)
	if outSnap > 0 || totalSnap > 0 {
		p2["peer_dialing"] = false
		delete(p2, "from_disk_snapshot")
		if act, _ := p2["dogego_sync_activity"].(map[string]any); act != nil {
			if h, _ := act["headline"].(string); h == "Connecting to peers" {
				p2["dogego_sync_activity"] = map[string]any{
					"headline": "Syncing blocks",
					"detail":   fmt.Sprintf("%d peer(s) connected — downloading and verifying block bodies.", outSnap),
				}
			}
		}
	}
	need := live["p2p"] == nil
	if !need {
		if cur, ok := live["p2p"].(map[string]any); ok {
			_, outLive, totalLive := p2pSnapConnectionCounts(cur)
			need = totalLive == 0 && outLive == 0 && (totalSnap > 0 || outSnap > 0)
			if !need {
				if act, _ := cur["dogego_sync_activity"].(map[string]any); act != nil {
					if h, _ := act["headline"].(string); h == "Connecting to peers" && outSnap > 0 {
						need = true
					}
				}
			}
		}
	}
	if !need {
		return nil
	}
	live["p2p"] = p2
	delete(live, "analytics_summary") // never re-embed fat analytics on the hot path
	if sum, ok := live["summary"].(map[string]any); ok {
		overlayP2PProgressOnSummary(sum, p2)
		live["summary"] = sum
	} else if len(sumB) > 0 {
		var sum map[string]any
		if json.Unmarshal(sumB, &sum) == nil {
			overlayP2PProgressOnSummary(sum, p2)
			live["summary"] = sum
		}
	}
	if p2PHasLiveProgress(p2) || p2PHasLiveConnections(p2) {
		delete(live, "from_disk_snapshot")
		delete(live, "summary_stale")
	}
	out, err := json.Marshal(live)
	if err != nil {
		return nil
	}
	return out
}

// overlayContiguousReconcileOnLiveJSON injects fresh startup reconcile progress into a
// cached /api/live payload so the dock bar moves between LiveFeed ticks.
func overlayContiguousReconcileOnLiveJSON(b []byte) []byte {
	st, ok := store.ContiguousReconcileProgress()
	if !ok || !st.Active {
		return nil
	}
	var live map[string]any
	if json.Unmarshal(b, &live) != nil || live == nil {
		return nil
	}
	sum, _ := live["summary"].(map[string]any)
	if sum == nil {
		sum = map[string]any{}
		live["summary"] = sum
	}
	if !applyContiguousReconcileLoading(sum) {
		attachContiguousReconcileNote(sum)
		out, err := json.Marshal(live)
		if err != nil {
			return nil
		}
		return out
	}
	live["warming_up"] = true
	out, err := json.Marshal(live)
	if err != nil {
		return nil
	}
	return out
}

// sanitizeLiveJSONIfP2PActive clears from_disk_snapshot / summary_stale when live IBD
// metrics are present so Overview does not freeze on "Showing last known data".
func sanitizeLiveJSONIfP2PActive(b []byte) []byte {
	var live map[string]any
	if json.Unmarshal(b, &live) != nil || live == nil {
		return nil
	}
	p2, _ := live["p2p"].(map[string]any)
	if !p2PHasLiveProgress(p2) {
		return nil
	}
	if p2 != nil && p2["from_disk_snapshot"] == true {
		delete(p2, "from_disk_snapshot")
	}
	sum, _ := live["summary"].(map[string]any)
	need := live["from_disk_snapshot"] == true || live["summary_stale"] == true
	if sum != nil && (sum["from_disk_snapshot"] == true || sum["summary_stale"] == true) {
		need = true
	}
	if sum != nil {
		if line, _ := sum["sync_status_line"].(string); strings.Contains(line, "last known data") {
			need = true
		}
	}
	if !need {
		return nil
	}
	delete(live, "from_disk_snapshot")
	delete(live, "summary_stale")
	if sum != nil {
		clearSummaryStaleFlags(sum)
		overlayP2PProgressOnSummary(sum, p2)
		if line, _ := sum["sync_status_line"].(string); strings.Contains(line, "last known data") {
			if h, _ := sum["dogego_sync_status"].(string); h != "" && !strings.Contains(h, "last known data") {
				sum["sync_status_line"] = h
			} else if act, _ := sum["dogego_sync_activity"].(map[string]any); act != nil {
				if h, _ := act["headline"].(string); h != "" {
					sum["sync_status_line"] = h
					sum["dogego_sync_status"] = h
				}
			}
		}
		live["summary"] = sum
	}
	if p2 != nil {
		live["p2p"] = p2
	}
	out, err := json.Marshal(live)
	if err != nil {
		return nil
	}
	return out
}

func toIntAny(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

// noteContiguousRate fills blk/min from contiguous tip deltas when ibd_progress overlay is missing.
func (f *LiveFeed) noteContiguousRate(sum map[string]any, cont int64) {
	if f == nil || sum == nil || cont < 0 {
		return
	}
	if bpm, ok := float64FromAny(sum["blocks_per_minute"]); ok && bpm > 0 {
		f.mu.Lock()
		f.contRateHeight = cont
		f.contRateAt = time.Now()
		f.mu.Unlock()
		return
	}
	f.mu.Lock()
	now := time.Now()
	prevH := f.contRateHeight
	prevAt := f.contRateAt
	if prevAt.IsZero() || cont < prevH {
		f.contRateHeight = cont
		f.contRateAt = now
		f.mu.Unlock()
		return
	}
	if cont == prevH {
		f.mu.Unlock()
		return
	}
	elapsed := now.Sub(prevAt).Minutes()
	f.contRateHeight = cont
	f.contRateAt = now
	f.mu.Unlock()
	if elapsed < 0.05 {
		return
	}
	rate := float64(cont-prevH) / elapsed
	if rate <= 0 {
		return
	}
	sum["contiguous_blocks_per_minute"] = rate
	sum["dogego_contiguous_blocks_per_minute"] = rate
	if _, ok := sum["blocks_per_minute"]; !ok {
		sum["blocks_per_minute"] = rate
	}
}

// ChainStatsCached returns light chainstats JSON, refreshing at most every minInterval.
func (f *LiveFeed) ChainStatsCached(cfg StartConfig, light bool, minInterval time.Duration) []byte {
	if cfg.ActiveJournal() == nil {
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
	stats := BuildChainStats(cfg.ActiveJournal(), cfg.ActiveRawBlocks(), cfg.PubkeyHashAddrID, now, chainActive, stored, light)
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
