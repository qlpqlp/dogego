// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"strconv"
	"sync"
	"time"

	"dogego/applog"
	"dogego/store"
)

var ibdUtxoSnapshotSave struct {
	mu              sync.Mutex
	lastSavedHeight int64
}

var connectCatchUpRunning sync.Mutex // held for duration of one catch-up tick (non-reentrant)

// InitIBDUtxoSnapshotFromTip seeds snapshot dedup from a loaded utxo.cache tip (startup).
func InitIBDUtxoSnapshotFromTip(tip int64) {
	if tip < 0 {
		return
	}
	ibdUtxoSnapshotSave.mu.Lock()
	if tip > ibdUtxoSnapshotSave.lastSavedHeight {
		ibdUtxoSnapshotSave.lastSavedHeight = tip
	}
	ibdUtxoSnapshotSave.mu.Unlock()
}

// MaybeSaveIBDUtxoSnapshot persists utxo.cache when chainActive advances (connect catch-up / shutdown).
func MaybeSaveIBDUtxoSnapshot(bs *BlockStoreCtx, utxo *store.UtxoCache, chainDir string, height int64) {
	if utxo == nil || chainDir == "" || height <= 0 {
		return
	}
	interval := int64(256)
	if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) && height < 50_000 {
		interval = 128
	}
	// IBD optimize: keep the UTXO set in RAM longer (Core -dbcache style) — fewer utxo.cache flushes.
	if bs != nil && bs.IBDOptimize && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		interval = 1024
		if height < 50_000 {
			interval = 512
		}
	}
	if bs != nil && utxo != nil && ConnectCatchUpLag(bs, utxo) > 8192 {
		interval = 512
		if bs.IBDOptimize {
			interval = 2048
		}
	}
	// Larger -dbcache budgets: flush less often by height (RAM holds more of the working set).
	if bs != nil && bs.DbCacheMB >= 2048 {
		if interval < 2048 {
			interval = 2048
		}
		if bs.DbCacheMB >= 8192 && interval < 4096 {
			interval = 4096
		}
	}
	overBudget := false
	if bs != nil && bs.DbCacheMB > 0 {
		budget := int64(bs.DbCacheMB) * 1024 * 1024
		if utxo.ApproxMemoryBytes() >= budget {
			overBudget = true
		}
	}
	if !overBudget && height%interval != 0 {
		return
	}
	if !BodiesAlignedForUtxoSnapshot(bs, utxo) {
		return
	}
	path := store.UtxoSnapshotPath(chainDir)
	if shouldSkipUtxoSnapshotDowngrade(path, height) {
		return
	}
	ibdUtxoSnapshotSave.mu.Lock()
	defer ibdUtxoSnapshotSave.mu.Unlock()
	if height <= ibdUtxoSnapshotSave.lastSavedHeight {
		return
	}
	started, err := utxo.SaveSnapshotAsync(path)
	if err != nil {
		applog.Line("utxo", "IBD utxo snapshot save: "+err.Error())
		return
	}
	if !started {
		return
	}
	ibdUtxoSnapshotSave.lastSavedHeight = height
	applog.Line("utxo", "IBD utxo snapshot save started through height "+strconv.FormatInt(height, 10))
}

var caughtUpUtxoSnapshotSave struct {
	mu         sync.Mutex
	lastHeight int64
	lastAt     time.Time
}

// MaybeSaveCaughtUpUtxoSnapshot persists utxo.cache during caught-up solo operation (debounced async).
func MaybeSaveCaughtUpUtxoSnapshot(bs *BlockStoreCtx, utxo *store.UtxoCache, chainDir string) {
	if bs == nil || utxo == nil || chainDir == "" || BodiesBehindHeaders(bs) {
		return
	}
	if ConnectCatchUpLag(bs, utxo) > 0 {
		return
	}
	h := utxo.TipHeight()
	if h < 0 || !BodiesAlignedForUtxoSnapshot(bs, utxo) {
		return
	}
	caughtUpUtxoSnapshotSave.mu.Lock()
	defer caughtUpUtxoSnapshotSave.mu.Unlock()
	if h == caughtUpUtxoSnapshotSave.lastHeight {
		return
	}
	if caughtUpUtxoSnapshotSave.lastHeight >= 0 && h-caughtUpUtxoSnapshotSave.lastHeight < 4 && time.Since(caughtUpUtxoSnapshotSave.lastAt) < 30*time.Second {
		return
	}
	path := store.UtxoSnapshotPath(chainDir)
	started, err := utxo.SaveSnapshotAsync(path)
	if err != nil {
		applog.Line("utxo", "caught-up utxo snapshot save: "+err.Error())
		return
	}
	if !started {
		return
	}
	caughtUpUtxoSnapshotSave.lastHeight = h
	caughtUpUtxoSnapshotSave.lastAt = time.Now()
	_ = store.SaveChainActiveManifest(chainDir, store.ChainActiveManifest{
		UtxoTipHeight:       h,
		ContiguousRawHeight: bs.ContiguousRawHeight(),
	})
	applog.Line("utxo", "caught-up utxo snapshot save started through height "+strconv.FormatInt(h, 10))
}

// startConnectCatchUpWorker replays ConnectBlock for stored bodies while chainActive lags during IBD.
// Core keeps validation active alongside block download; the main P2P read loop can starve periodic catch-up.
func startConnectCatchUpWorker(ctx context.Context, bs *BlockStoreCtx, utxo *store.UtxoCache) {
	if bs == nil || utxo == nil {
		return
	}
	go func() {
		// Post-restart burst after UTXO snapshot restore (lag can appear seconds after worker start).
		go func() {
			deadline := time.Now().Add(3 * time.Minute)
			for time.Now().Before(deadline) {
				if bs == nil || utxo == nil || !BodiesBehindHeaders(bs) {
					time.Sleep(500 * time.Millisecond)
					continue
				}
				if ConnectCatchUpLag(bs, utxo) >= 2048 {
					if connectCatchUpRunning.TryLock() {
						runConnectCatchUpStartupBurst(bs, utxo)
						connectCatchUpRunning.Unlock()
					}
					return
				}
				time.Sleep(time.Second)
			}
		}()
		interval := 100 * time.Millisecond
		for {
			if ShouldDeferConnectForBodyDownload(bs) {
				noteConnectDeferredForDownload(bs)
				interval = 5 * time.Second
			} else if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
				interval = 250 * time.Millisecond
			} else {
				interval = 100 * time.Millisecond
			}
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if ShouldDeferConnectForBodyDownload(bs) {
				continue
			}
			if !connectCatchUpRunning.TryLock() {
				continue
			}
			runConnectCatchUpOnce(bs, utxo)
			connectCatchUpRunning.Unlock()
		}
	}()
	applog.Line("utxo", "connect catch-up worker started (download-first IBD: ConnectBlock after bodies catch headers)")
}

// startIBDConnectWorkers runs connect catch-up as soon as blockStore/UTXO are ready (Core: validation alongside download).
func startIBDConnectWorkers(ctx context.Context, bs *BlockStoreCtx, utxo *store.UtxoCache, quarantineOnStartup bool) {
	if bs == nil || utxo == nil {
		return
	}
	startConnectCatchUpWorker(ctx, bs, utxo)
	startReplayRampWorker(ctx, bs)
	if quarantineOnStartup {
		go rebuildUtxoFromStoredBodiesAfterQuarantine(bs, utxo)
	}
}
