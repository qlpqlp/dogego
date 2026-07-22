// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"dogego/applog"
	"dogego/consensus"
	"dogego/store"
)

const (
	connectCatchUpPollInterval   = 3 * time.Second
	connectCatchUpMinLag         = 128
	connectCatchUpMinLagDeepIBD  = 32  // connect sooner while header getheaders is paused (mid IBD)
	connectCatchUpMinLagFrontier   = 1  // connect as soon as stored bodies cover chainActive+1 during early deep body IBD
	connectCatchUpMinLagCaughtUp   = 1  // connect when headers caught up but chainActive lags stored bodies (solo restart)
	connectCatchUpPollCaughtUp     = 2 * time.Second
)

// EffectiveConnectCatchUpMinLag returns how far stored bodies may run ahead of chainActive
// before dedicated connect catch-up runs (lower during deep forward body IBD).
func EffectiveConnectCatchUpMinLag(bs *BlockStoreCtx) int64 {
	if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		if cont := bs.ContiguousRawHeight(); cont >= 0 && cont < 50_000 {
			return connectCatchUpMinLagFrontier
		}
		return connectCatchUpMinLagDeepIBD
	}
	return connectCatchUpMinLag
}

// connectFrontierScriptsEnabled reports whether the next ConnectBlock height runs full script checks.
func connectFrontierScriptsEnabled(bs *BlockStoreCtx) bool {
	if bs == nil || bs.Utxo == nil {
		return true
	}
	next := bs.Utxo.TipHeight() + 1
	if next < 0 {
		next = 0
	}
	return consensus.ScriptChecksEnabledAtHeight(next)
}

// connectCatchUpBlocksPerIBDCall caps ConnectBlock work per SyncUtxoCache during body IBD.
func connectCatchUpBlocksPerIBDCall(bs *BlockStoreCtx) int {
	if bs == nil || bs.Utxo == nil {
		return 8
	}
	paused := ShouldPauseHeaderCatchUpForBodyIBD(bs, 0)
	lag := ConnectCatchUpLag(bs, bs.Utxo)
	small := 4
	if paused {
		small = 16
	}
	if paused && lag > 512 {
		if n := connectFrontierMaxSteps(bs); n > small {
			cap := 32
			switch {
			case lag > 8192:
				cap = 128
			case lag > 2048:
				cap = 64
			}
			if !connectFrontierScriptsEnabled(bs) {
				switch {
				case lag > 8192:
					cap = 512
				case lag > 2048:
					cap = 256
				}
			}
			if n > cap {
				n = cap
			}
			return n
		}
	}
	switch {
	case lag > 8192:
		return 16
	case lag > 4096:
		return 12
	case lag > 2048:
		return 8
	default:
		return small
	}
}

// connectCatchUpPasses scales SyncUtxoCache rounds per wake when stored bodies outpace chainActive.
func connectCatchUpPasses(lag int64, bs *BlockStoreCtx) int {
	var passes int
	switch {
	case lag > 8192:
		passes = 4
	case lag > 4096:
		passes = 3
	case lag > 2048:
		passes = 2
	default:
		passes = 1
	}
	if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		if lag > 8192 && passes < 8 {
			passes = 8
		} else if lag > 512 && passes < 4 {
			passes = 4
		} else if passes < 3 {
			passes++
		}
		if !connectFrontierScriptsEnabled(bs) {
			if lag > 8192 && passes < 16 {
				passes = 16
			} else if lag > 2048 && passes < 8 {
				passes = 8
			}
		}
	}
	return passes
}

// runConnectCatchUpStartupBurst replays ConnectBlock aggressively after restart when stored bodies far exceed chainActive.
func runConnectCatchUpStartupBurst(bs *BlockStoreCtx, utxo *store.UtxoCache) {
	if bs == nil || utxo == nil || !BodiesBehindHeaders(bs) {
		return
	}
	lag := ConnectCatchUpLag(bs, utxo)
	if lag < 2048 {
		runConnectCatchUpOnce(bs, utxo)
		return
	}
	const maxBursts = 32
	before := utxo.TipHeight()
	for i := 0; i < maxBursts; i++ {
		prev := utxo.TipHeight()
		runConnectCatchUpOnce(bs, utxo)
		if ConnectCatchUpLag(bs, utxo) < EffectiveConnectCatchUpMinLag(bs) {
			break
		}
		if utxo.TipHeight() == prev {
			break
		}
	}
	after := utxo.TipHeight()
	if after > before {
		applog.Line("utxo", "connect catch-up startup burst: chainActive "+strconv.FormatInt(before, 10)+
			" → "+strconv.FormatInt(after, 10)+fmt.Sprintf(" (lag %d)", ConnectCatchUpLag(bs, utxo)))
	}
}

func runConnectCatchUpOnce(bs *BlockStoreCtx, utxo *store.UtxoCache) {
	if bs == nil || utxo == nil {
		return
	}
	if !BodiesBehindHeaders(bs) {
		runCaughtUpConnectLagOnce(bs, utxo)
		return
	}
	lag := ConnectCatchUpLag(bs, utxo)
	minLag := EffectiveConnectCatchUpMinLag(bs)
	if lag < minLag {
		if bs.utxoAheadOfStoredBodies() {
			after := rampReplayContiguousFromDiskBounded(bs, 8)
			if after >= 0 {
				RecordIBDConnectAdvance(after)
			}
		}
		return
	}
	before := utxo.TipHeight()
	passes := connectCatchUpPasses(lag, bs)
	var err error
	for i := 0; i < passes; i++ {
		if err = bs.SyncUtxoCache(); err != nil {
			break
		}
		if ConnectCatchUpLag(bs, utxo) < EffectiveConnectCatchUpMinLag(bs) {
			break
		}
	}
	if err != nil {
		bs.maybeRealignBodyDownloadOnConnectGap()
		maybeRepairTxIndexOnConnectStall(bs, err)
		return
	}
	after := utxo.TipHeight()
	if after > before && after >= 0 {
		RecordIBDConnectAdvance(after)
		dh := after - before
		if dh >= 4 || after%64 == 0 {
			applog.Line("utxo", "connect catch-up: chainActive "+strconv.FormatInt(before, 10)+
				" → "+strconv.FormatInt(after, 10)+fmt.Sprintf(" (+%d, lag %d)", dh, ConnectCatchUpLag(bs, utxo)))
		}
		if after > 0 && after%256 == 0 {
			if dir := blockStoreChainDir(bs); dir != "" {
				go MaybeSaveIBDUtxoSnapshot(bs, utxo, dir, after)
			}
		}
	}
}

// connectCatchUpInterval tightens connect replay while stored bodies run far ahead of chainActive (Core catch-up).
func connectCatchUpInterval(lag int64) time.Duration {
	switch {
	case lag > 8192:
		return 500 * time.Millisecond
	case lag > 2048:
		return time.Second
	case lag > 512:
		return 2 * time.Second
	default:
		return connectCatchUpPollInterval
	}
}

var ibdConnectRate struct {
	mu      sync.Mutex
	samples []connectRateSample
}

type connectRateSample struct {
	at     time.Time
	height int64
}

// RecordIBDConnectAdvance tracks chainActive height for connect throughput metrics.
func RecordIBDConnectAdvance(height int64) {
	if height < 0 {
		return
	}
	now := time.Now()
	ibdConnectRate.mu.Lock()
	defer ibdConnectRate.mu.Unlock()
	ibdConnectRate.samples = append(ibdConnectRate.samples, connectRateSample{at: now, height: height})
	cutoff := now.Add(-5 * time.Minute)
	i := 0
	for i < len(ibdConnectRate.samples) && ibdConnectRate.samples[i].at.Before(cutoff) {
		i++
	}
	ibdConnectRate.samples = ibdConnectRate.samples[i:]
}

// IBDConnectBlocksPerMinute estimates recent chainActive advance rate from connect samples.
func IBDConnectBlocksPerMinute() float64 {
	ibdConnectRate.mu.Lock()
	defer ibdConnectRate.mu.Unlock()
	if len(ibdConnectRate.samples) < 2 {
		return 0
	}
	first := ibdConnectRate.samples[0]
	last := ibdConnectRate.samples[len(ibdConnectRate.samples)-1]
	dh := last.height - first.height
	if dh <= 0 {
		return 0
	}
	elapsed := last.at.Sub(first.at)
	if elapsed < 15*time.Second {
		return 0
	}
	return float64(dh) / elapsed.Minutes()
}

// ConnectCatchUpLag is stored contiguous bodies ahead of chainActive (UTXO tip).
func ConnectCatchUpLag(bs *BlockStoreCtx, utxo *store.UtxoCache) int64 {
	if bs == nil || utxo == nil {
		return 0
	}
	cont := bs.ContiguousRawHeight()
	if cont < 0 {
		return 0
	}
	tip := utxo.TipHeight()
	if tip < 0 {
		return cont + 1
	}
	lag := cont - tip
	if lag <= 0 {
		return 0
	}
	return lag
}

func caughtUpConnectMaxBlocks(lag int64) int {
	switch {
	case lag > 512:
		return 128
	case lag > 64:
		return 32
	case lag < 1:
		return 1
	default:
		return int(lag)
	}
}

// runCaughtUpConnectLagOnce drains small stored-body ahead of chainActive when headers are caught up.
func runCaughtUpConnectLagOnce(bs *BlockStoreCtx, utxo *store.UtxoCache) {
	if bs == nil || utxo == nil || BodiesBehindHeaders(bs) {
		return
	}
	lag := ConnectCatchUpLag(bs, utxo)
	if lag < connectCatchUpMinLagCaughtUp {
		return
	}
	before := utxo.TipHeight()
	if err := bs.SyncUtxoCacheBounded(caughtUpConnectMaxBlocks(lag)); err != nil {
		applog.Line("utxo", "caught-up connect: "+err.Error())
		return
	}
	after := utxo.TipHeight()
	if after > before && after >= 0 {
		applog.Line("utxo", "caught-up connect: chainActive "+strconv.FormatInt(before, 10)+
			" → "+strconv.FormatInt(after, 10)+fmt.Sprintf(" (lag %d)", ConnectCatchUpLag(bs, utxo)))
		if dir := blockStoreChainDir(bs); dir != "" {
			MaybeSaveCaughtUpUtxoSnapshot(bs, utxo, dir)
		}
	}
}

// MaybeSyncConnectCatchUp connects stored blocks when chainActive lags during IBD or after catch-up restart.
// Bodies may pause while thousands of stored blocks remain unconnected (e.g. after restart).
func MaybeSyncConnectCatchUp(bs *BlockStoreCtx, utxo *store.UtxoCache, last *time.Time) {
	if bs == nil || utxo == nil || last == nil {
		return
	}
	if !BodiesBehindHeaders(bs) {
		lag := ConnectCatchUpLag(bs, utxo)
		if lag < connectCatchUpMinLagCaughtUp {
			return
		}
		if !last.IsZero() && time.Since(*last) < connectCatchUpPollCaughtUp {
			return
		}
		*last = time.Now()
		runCaughtUpConnectLagOnce(bs, utxo)
		return
	}
	if ConnectCatchUpLag(bs, utxo) < EffectiveConnectCatchUpMinLag(bs) {
		return
	}
	lag := ConnectCatchUpLag(bs, utxo)
	interval := connectCatchUpInterval(lag)
	if !last.IsZero() && time.Since(*last) < interval {
		return
	}
	*last = time.Now()
	before := utxo.TipHeight()
	passes := connectCatchUpPasses(lag, bs)
	var err error
	for i := 0; i < passes; i++ {
		if err = bs.SyncUtxoCache(); err != nil {
			break
		}
		if ConnectCatchUpLag(bs, utxo) < EffectiveConnectCatchUpMinLag(bs) {
			break
		}
	}
	if err != nil {
		applog.Line("utxo", "connect catch-up: "+err.Error())
		bs.maybeRealignBodyDownloadOnConnectGap()
		maybeRepairTxIndexOnConnectStall(bs, err)
		if isConnectStallErr(err) {
			if err2 := bs.SyncUtxoCache(); err2 == nil {
				after := utxo.TipHeight()
				if after > before && after >= 0 {
					RecordIBDConnectAdvance(after)
					applog.Line("utxo", "connect catch-up: chainActive "+strconv.FormatInt(before, 10)+" → "+strconv.FormatInt(after, 10)+
						" (stored through "+strconv.FormatInt(bs.ContiguousRawHeight(), 10)+", after txindex repair)")
				}
				return
			}
		}
		return
	}
	after := utxo.TipHeight()
	if after > before && after >= 0 {
		RecordIBDConnectAdvance(after)
		applog.Line("utxo", "connect catch-up: chainActive "+strconv.FormatInt(before, 10)+" → "+strconv.FormatInt(after, 10)+
			" (stored through "+strconv.FormatInt(bs.ContiguousRawHeight(), 10)+")")
	}
}

// connectFrontierMaxSteps scales per-call connect batch when chainActive lags stored bodies.
func connectFrontierMaxSteps(bs *BlockStoreCtx) int {
	const base = 512
	if bs == nil || bs.Utxo == nil {
		return base
	}
	lag := ConnectCatchUpLag(bs, bs.Utxo)
	switch {
	case lag > 8192:
		return 4096
	case lag > 2048:
		return 4096
	default:
		return base
	}
}

// replayContiguousMaxSteps caps sequential disk advance during UTXO-snapshot body replay.
func replayContiguousMaxSteps(contiguous int64, utxo *store.UtxoCache) int {
	const base = 512
	if utxo == nil || contiguous < 0 {
		return base
	}
	remain := utxo.TipHeight() - contiguous
	if remain > 4096 {
		return 4096
	}
	if remain > int64(base) {
		return base
	}
	if remain <= 0 {
		return base
	}
	return int(remain)
}

func replayContiguousMaxStepsFor(bs *BlockStoreCtx) int {
	if bs == nil {
		return 512
	}
	bs.contiguousMu.Lock()
	cont := bs.contiguousTip
	utxo := bs.Utxo
	bs.contiguousMu.Unlock()
	return replayContiguousMaxSteps(cont, utxo)
}

// rampReplayContiguousFromDiskBounded runs multiple AdvanceReplayContiguousFromDisk passes
// per wake when parallel fetch stored bodies ahead of cached contiguous (UTXO snapshot replay).
func rampReplayContiguousFromDiskBounded(bs *BlockStoreCtx, maxPasses int) int64 {
	if bs == nil || maxPasses <= 0 {
		return -1
	}
	if !bs.utxoAheadOfStoredBodies() {
		return bs.ContiguousRawHeight()
	}
	start := bs.ContiguousRawHeight()
	for i := 0; i < maxPasses; i++ {
		before := bs.ContiguousRawHeight()
		after := bs.AdvanceReplayContiguousFromDisk(replayContiguousMaxStepsFor(bs))
		if after <= before {
			break
		}
	}
	cont := bs.ContiguousRawHeight()
	if start >= 0 && cont > start {
		applog.Line("block", fmt.Sprintf("replay ramp batch: contiguous %d → %d", start, cont))
	}
	return cont
}

// syncUtxoMaxConnectPasses scales multi-pass connect replay during deep catch-up.
func syncUtxoMaxConnectPasses(bs *BlockStoreCtx, contiguousThrough int64) int {
	maxConnectPasses := 128
	if !BodiesBehindHeaders(bs) || bs.Utxo == nil {
		return maxConnectPasses
	}
	utxoTip := bs.Utxo.TipHeight()
	if utxoTip < 0 {
		return maxConnectPasses
	}
	backlog := contiguousThrough - utxoTip
	switch {
	case backlog > 8192:
		return 512
	case backlog > 2048:
		return 384
	case backlog > 512:
		return 256
	default:
		return maxConnectPasses
	}
}

// PostBatchConnectLagThreshold triggers inline SyncUtxoCache after a getdata batch when stored bodies run this far ahead of chainActive.
func PostBatchConnectLagThreshold(bs *BlockStoreCtx) int64 {
	minLag := EffectiveConnectCatchUpMinLag(bs)
	if minLag < connectCatchUpMinLag {
		if minLag <= connectCatchUpMinLagFrontier {
			return connectCatchUpMinLagFrontier
		}
		return minLag * 4
	}
	return 512
}

// shouldPostBatchInlineConnect reports whether to run SyncUtxoCache immediately after a getdata batch.
// Deep body IBD prefers download: leave routine connect to the catch-up worker unless lag is extreme.
func shouldPostBatchInlineConnect(bs *BlockStoreCtx) bool {
	if bs == nil || bs.Utxo == nil {
		return false
	}
	if ConnectBodyGapHeight(bs) >= 0 {
		return false
	}
	lag := ConnectCatchUpLag(bs, bs.Utxo)
	if lag <= 0 {
		return false
	}
	if ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		// Prefer getdata throughput during deep body IBD; catch-up worker handles connect.
		// Aggressive inline connect here starves download and collapses blk/min after the first hour.
		thresh := int64(8192)
		if bs.IBDOptimize {
			thresh = 32768
		}
		return lag > thresh
	}
	return lag > PostBatchConnectLagThreshold(bs)
}

// formatConnectCatchUpNote returns a short operator-facing lag summary (tests / diagnostics).
func formatConnectCatchUpNote(bs *BlockStoreCtx, utxo *store.UtxoCache) string {
	lag := ConnectCatchUpLag(bs, utxo)
	if lag <= 0 {
		return ""
	}
	return fmt.Sprintf("connect_lag=%d", lag)
}
