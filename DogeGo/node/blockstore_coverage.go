// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"strings"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

const (
	blockReconnectWindow      = 64
	ibdConnectDeferInterval       = 256 // connect frontier at most this often during assume-valid bulk IBD
	ibdConnectDeferIntervalDeepIBD = 64  // tighter during deep body IBD when header getheaders is paused
	utxoSyncIntervalBulkIBD     = 512
	utxoSyncIntervalNormal      = 64
)

func (c *BlockStoreCtx) noteBlockStoredAt(height int64) {
	c.noteBlockStoredAtInner(height, true)
}

// noteBlockStoredAtDeferred updates contiguous coverage without UTXO/filter/index side effects
// (used during batched P2P fetch so the read loop is not blocked on heavy IBD hooks).
func (c *BlockStoreCtx) noteBlockStoredAtDeferred(height int64) {
	c.noteBlockStoredAtInner(height, false)
}

func (c *BlockStoreCtx) noteBlockStoredAtInner(height int64, notify bool) {
	if c == nil || height < 0 {
		return
	}
	var notifyCont int64 = -1
	c.contiguousMu.Lock()
	replay := c.utxoSnapshotReplayLocked()
	if c.contiguousTip < 0 && !replay {
		c.recomputeContiguousTipLocked()
	}
	prev := c.contiguousTip
	if height == c.contiguousTip+1 {
		if c.Journal != nil && c.Raw != nil &&
			store.HasStoredBodyAtHeight(c.Journal, c.Raw, height, c.chainNet()) {
			c.contiguousTip = height
			if !replay {
				c.extendContiguousForwardLocked()
			}
		}
	} else if height <= c.contiguousTip && !replay {
		c.extendContiguousForwardLocked()
	}
	// height > contiguous+1: parallel getdata may land ahead of the hole.
	// Do not rescan from genesis â€” that hashed+stated every stored height on
	// each file and froze every download lane. Coverage moves when the hole fills.
	if notify && c.onContiguousAdvance != nil && c.contiguousTip >= 0 && c.contiguousTip != prev {
		notifyCont = c.contiguousTip
	}
	if replay {
		c.advanceReplayContiguousLocked(replayContiguousMaxSteps(c.contiguousTip, c.Utxo))
		if c.onContiguousAdvance != nil && c.contiguousTip >= 0 && c.contiguousTip != prev {
			notifyCont = c.contiguousTip
		}
	}
	c.contiguousMu.Unlock()
	if notifyCont >= 0 && c.onContiguousAdvance != nil {
		c.onContiguousAdvance(notifyCont)
	}
	if notify {
		c.maybeBackfillAuxNearActivation(height)
	}
}

// maybeBackfillAuxNearActivation fills headers_aux.bin from raw blocks around the auxpow fork (mainnet 371337).
// Scan height is capped to activation+window or contiguous+window (not the full header tip).
func (c *BlockStoreCtx) maybeBackfillAuxNearActivation(height int64) {
	if c == nil || c.Aux == nil || c.Raw == nil || c.Journal == nil {
		return
	}
	activation := consensus.AuxpowActivationHeight(c.Params.Net)
	if activation <= 0 || c.Params.Net != chain.MainnetDogecoin {
		return
	}
	const window = 256
	if height < activation || height > activation+window {
		return
	}
	if height != activation && height != activation+1 && height%32 != 0 {
		return
	}
	cont := c.ContiguousRawHeight()
	through := activation + window
	if cont >= activation {
		if cont+window > through {
			through = cont + window
		}
	}
	tip, err := c.Journal.TipHeight()
	if err != nil {
		return
	}
	if through > tip {
		through = tip
	}
	n, err := store.BackfillAuxThroughHeight(c.Journal, c.Aux, c.Raw, through)
	if err != nil {
		applog.Line("headers", "aux backfill near activation: "+err.Error())
		return
	}
	if n > 0 {
		applog.Line("headers", fmt.Sprintf("aux backfill near activation: %d auxpow record(s) filled (through height %d, header tip %d)", n, through, tip))
	}
}

// maybeBackfillAuxAfterHeaderAdvance fills auxpow slots from raw blocks after header tip moves forward.
func (c *BlockStoreCtx) maybeBackfillAuxAfterHeaderAdvance() {
	if c == nil || c.Aux == nil || c.Raw == nil || c.Journal == nil {
		return
	}
	tip, err := c.Journal.TipHeight()
	if err != nil || tip < 0 {
		return
	}
	through := tip
	if cont := c.ContiguousRawHeight(); cont >= 0 && tip-cont > 2048 {
		through = cont + 2048
	}
	n, err := store.BackfillAuxThroughHeight(c.Journal, c.Aux, c.Raw, through)
	if err != nil {
		applog.Line("headers", "aux backfill after header catch-up: "+err.Error())
		return
	}
	if n > 0 {
		applog.Line("headers", fmt.Sprintf("aux backfill after header catch-up: %d auxpow record(s) (through %d, tip %d)", n, through, tip))
	}
}

// extendContiguousIfNextStored walks cached coverage forward while contiguous+1 is on disk.
// Used instead of scanning the header tip for the next hole (O(headers) Stats during IBD).
func (c *BlockStoreCtx) extendContiguousIfNextStored() {
	if c == nil {
		return
	}
	c.contiguousMu.Lock()
	defer c.contiguousMu.Unlock()
	if c.contiguousTip < 0 {
		return
	}
	c.extendContiguousForwardLocked()
}

// extendContiguousForwardLocked extends contiguousTip while consecutive headers have raw bodies.
func (c *BlockStoreCtx) extendContiguousForwardLocked() {
	if c == nil || c.Journal == nil || c.Raw == nil || c.contiguousTip < 0 {
		return
	}
	if c.utxoAheadOfStoredBodiesAt(c.contiguousTip) {
		return
	}
	for {
		next := c.contiguousTip + 1
		if _, err := c.Journal.ReadHeaderAt(next); err != nil {
			return
		}
		if !store.HasStoredBodyAtHeight(c.Journal, c.Raw, next, c.chainNet()) {
			return
		}
		c.contiguousTip = next
	}
}

// advanceReplayContiguousLocked extends cached coverage by one stored body at a time during
// UTXO-snapshot replay (parallel fetch may store bodies ahead of contiguous+1).
// Caller must hold contiguousMu.
func (c *BlockStoreCtx) advanceReplayContiguousLocked(maxSteps int) {
	if c == nil || c.Journal == nil || c.Raw == nil || maxSteps <= 0 || c.contiguousTip < 0 {
		return
	}
	if !c.utxoSnapshotReplayLocked() {
		return
	}
	for step := 0; step < maxSteps; step++ {
		next := c.contiguousTip + 1
		if c.Utxo != nil && next > c.Utxo.TipHeight() {
			return
		}
		if _, err := c.Journal.ReadHeaderAt(next); err != nil {
			return
		}
		if !store.HasStoredBodyAtHeight(c.Journal, c.Raw, next, c.chainNet()) {
			return
		}
		c.contiguousTip = next
	}
}

// TrySeedContiguousFromCheckpoint restores monotonic raw coverage from rawblocks_sync.json during
// UTXO-snapshot replay. A full genesis scan stops at ancient stub gaps below the replay frontier.
func (c *BlockStoreCtx) TrySeedContiguousFromCheckpoint(height int64) bool {
	if c == nil || height < 0 || c.Journal == nil || c.Raw == nil {
		return false
	}
	if !store.HasStoredBodyAtHeight(c.Journal, c.Raw, height, c.chainNet()) {
		return false
	}
	if height > 0 && !store.HasStoredBodyAtHeight(c.Journal, c.Raw, height-1, c.chainNet()) {
		return false
	}
	c.contiguousMu.Lock()
	seeded := height > c.contiguousTip
	if seeded {
		c.contiguousTip = height
	}
	c.contiguousMu.Unlock()
	return seeded
}

func (c *BlockStoreCtx) seedReplayContiguousIfUnsetLocked() {
	if c == nil || c.contiguousTip >= 0 || c.Journal == nil || c.Raw == nil {
		return
	}
	if !c.utxoSnapshotReplayLocked() {
		return
	}
	if store.HasStoredBodyAtHeight(c.Journal, c.Raw, 0, c.chainNet()) {
		c.contiguousTip = 0
	}
}

// RampReplayContiguousFromDisk advances cached contiguous from disk through the UTXO snapshot
// tip in bounded batches (startup / stall recovery when bodies already exist on disk).
func (c *BlockStoreCtx) RampReplayContiguousFromDisk() int64 {
	if c == nil {
		return -1
	}
	if !c.utxoAheadOfStoredBodies() {
		return c.ContiguousRawHeight()
	}
	start := c.ContiguousRawHeight()
	for {
		before := c.ContiguousRawHeight()
		after := c.AdvanceReplayContiguousFromDisk(replayContiguousMaxStepsFor(c))
		if after <= before {
			break
		}
	}
	cont := c.ContiguousRawHeight()
	if start >= 0 && cont > start {
		applog.Line("block", fmt.Sprintf("replay ramp: contiguous %d â†’ %d (disk advance)", start, cont))
	} else if start < 0 && cont >= 0 {
		applog.Line("block", fmt.Sprintf("replay ramp: contiguous â†’ %d (disk advance)", cont))
	}
	return cont
}

// AdvanceReplayContiguousFromDisk moves cached contiguous forward while height contiguous+1
// has an adequate stored body (UTXO-snapshot body replay only).
func (c *BlockStoreCtx) AdvanceReplayContiguousFromDisk(maxSteps int) int64 {
	if c == nil || maxSteps <= 0 {
		return c.ContiguousRawHeight()
	}
	c.contiguousMu.Lock()
	prev := c.contiguousTip
	if c.contiguousTip < 0 {
		if c.utxoSnapshotReplayLocked() {
			c.seedReplayContiguousIfUnsetLocked()
		} else {
			c.recomputeContiguousTipLocked()
		}
	}
	c.advanceReplayContiguousLocked(maxSteps)
	cont := c.contiguousTip
	fn := c.onContiguousAdvance
	c.contiguousMu.Unlock()
	if fn != nil && cont > prev {
		fn(cont)
	}
	return cont
}

// SeedContiguousTip sets cached raw-body coverage (e.g. after Core blk index reports full chain).
func (c *BlockStoreCtx) SeedContiguousTip(height int64) {
	if c == nil || height < 0 {
		return
	}
	c.contiguousMu.Lock()
	defer c.contiguousMu.Unlock()
	if height > c.contiguousTip {
		c.contiguousTip = height
	}
}

// RevalidateContiguousTip rescans disk from genesis and may shrink cached contiguous coverage
// after a body is removed below the previous tip.
func (c *BlockStoreCtx) RevalidateContiguousTip() int64 {
	if c == nil {
		return -1
	}
	c.contiguousMu.Lock()
	prev := c.contiguousTip
	c.recomputeContiguousTipLocked()
	cont := c.contiguousTip
	fn := c.onContiguousAdvance
	c.contiguousMu.Unlock()
	if fn != nil && cont > prev {
		fn(cont)
	}
	return cont
}

// maybeClampBundledContiguousFromDisk lowers cached contiguous coverage when bundled blk*.dat
// probe shows a torn tail below the in-memory tip (kill-mid-append recovery).
func (c *BlockStoreCtx) maybeClampBundledContiguousFromDisk() int64 {
	if c == nil || c.Raw == nil {
		return -1
	}
	if c.Raw.StorageOpts().Layout != store.BlockLayoutBundled {
		return c.ContiguousRawHeight()
	}
	diskTip, err := c.Raw.ProbeBundledContiguousTip()
	if err != nil || diskTip < 0 {
		return c.ContiguousRawHeight()
	}
	c.contiguousMu.Lock()
	prev := c.contiguousTip
	if c.contiguousTip > diskTip {
		c.contiguousTip = diskTip
	}
	cont := c.contiguousTip
	c.contiguousMu.Unlock()
	if prev >= 0 && cont >= 0 && cont < prev {
		applog.Line("recovery", fmt.Sprintf("bundled contiguous clamp %d â†’ %d from disk probe", prev, cont))
	}
	return cont
}

// utxoAheadOfStoredBodies reports a UTXO snapshot loaded past readable raw-body coverage.
func (c *BlockStoreCtx) utxoAheadOfStoredBodies() bool {
	if c == nil || c.Utxo == nil {
		return false
	}
	cont := c.ContiguousRawHeight()
	return c.utxoAheadOfStoredBodiesAt(cont)
}

func (c *BlockStoreCtx) utxoAheadOfStoredBodiesAt(cont int64) bool {
	if c == nil || c.Utxo == nil {
		return false
	}
	if cont < 0 {
		return c.Utxo.TipHeight() >= 0
	}
	return c.Utxo.TipHeight() > cont
}

// utxoSnapshotReplayLocked reports UTXO-snapshot-ahead body replay (caller holds contiguousMu).
func (c *BlockStoreCtx) utxoSnapshotReplayLocked() bool {
	if c == nil || c.Utxo == nil {
		return false
	}
	if c.Utxo.TipHeight() < 0 {
		return false
	}
	if c.contiguousTip < 0 {
		return true
	}
	return c.Utxo.TipHeight() > c.contiguousTip
}

// shrinkContiguousTipAfterBodyRemoved lowers cached coverage when a purge creates a gap at
// or just ahead of the connect frontier. Purges far below contiguous+1 do not move the tip
// (avoids replay regressions when stub batches touch ancient heights).
func (c *BlockStoreCtx) shrinkContiguousTipAfterBodyRemoved(height int64) {
	if c == nil || height < 0 {
		return
	}
	c.contiguousMu.Lock()
	defer c.contiguousMu.Unlock()
	if height > c.contiguousTip+1 {
		return
	}
	// During snapshot replay, stall recovery may purge corrupt stubs at ancient heights
	// while cached contiguous reflects bodies stored far ahead - do not rewind the cache.
	if c.utxoSnapshotReplayLocked() && height < c.contiguousTip {
		return
	}
	if c.contiguousTip >= height {
		c.contiguousTip = height - 1
	}
}

// RefreshContiguousTip extends or recomputes cached contiguous coverage from disk.
// Call during IBD when parallel lanes may have stored bodies ahead of the cached tip.
func (c *BlockStoreCtx) RefreshContiguousTip() int64 {
	if c == nil {
		return -1
	}
	c.contiguousMu.Lock()
	prev := c.contiguousTip
	if c.contiguousTip < 0 {
		if c.utxoSnapshotReplayLocked() {
			c.seedReplayContiguousIfUnsetLocked()
		} else {
			c.recomputeContiguousTipLocked()
		}
	} else if c.utxoSnapshotReplayLocked() {
		c.advanceReplayContiguousLocked(replayContiguousMaxSteps(c.contiguousTip, c.Utxo))
	} else if !c.utxoAheadOfStoredBodiesAt(c.contiguousTip) {
		c.extendContiguousForwardLocked()
	}
	cont := c.contiguousTip
	fn := c.onContiguousAdvance
	c.contiguousMu.Unlock()
	if fn != nil && cont > prev {
		fn(cont)
	}
	return cont
}

// ContiguousRawHeight returns the highest height with stored raw blocks for every height in [0,h].
// Returns -1 when unknown or no blocks stored yet.
func (c *BlockStoreCtx) ContiguousRawHeight() int64 {
	if c == nil {
		return -1
	}
	c.contiguousMu.Lock()
	defer c.contiguousMu.Unlock()
	if c.contiguousTip < 0 {
		c.recomputeContiguousTipLocked()
	}
	return c.contiguousTip
}

// ResetContiguousTip clears cached raw-body coverage (after a header reorg prune).
func (c *BlockStoreCtx) ResetContiguousTip() {
	if c == nil {
		return
	}
	if ShouldPreserveContiguousCache(c) {
		return
	}
	c.contiguousMu.Lock()
	c.contiguousTip = -1
	c.contiguousMu.Unlock()
}

func (c *BlockStoreCtx) recomputeContiguousTipLocked() {
	if c == nil || c.Journal == nil || c.Raw == nil {
		if c != nil {
			c.contiguousTip = -1
		}
		return
	}
	// Scan into a local so concurrent ContiguousRawHeight readers never see -1
	// mid-pass (that used to trigger nested genesis scans on every claim).
	found := int64(-1)
	for h := int64(0); ; h++ {
		if _, err := c.Journal.ReadHeaderAt(h); err != nil {
			break
		}
		if !store.HasStoredBodyAtHeight(c.Journal, c.Raw, h, c.chainNet()) {
			break
		}
		found = h
	}
	c.contiguousTip = found
}

// hasAncestorBlockBodies reports whether raw block files exist for heights [0, height).
func (c *BlockStoreCtx) hasAncestorBlockBodies(height int64) bool {
	if c == nil || c.Journal == nil || c.Raw == nil {
		return false
	}
	if height <= 0 {
		return true
	}
	c.contiguousMu.Lock()
	if c.contiguousTip < 0 {
		c.recomputeContiguousTipLocked()
	}
	fast := c.contiguousTip >= 0 && c.contiguousTip >= height-1
	if fast {
		if !store.HasStoredBodyAtHeight(c.Journal, c.Raw, height-1, c.chainNet()) {
			fast = false
			c.recomputeContiguousTipLocked()
		}
	}
	c.contiguousMu.Unlock()
	if fast {
		return true
	}
	for h := height - 1; h >= 0; h-- {
		if !store.HasStoredBodyAtHeight(c.Journal, c.Raw, h, c.chainNet()) {
			return false
		}
	}
	return true
}

// ConnectBlockAtPayload runs Core ConnectBlock validation on serialized block bytes.
func (c *BlockStoreCtx) ConnectBlockAtPayload(payload []byte, height int64) error {
	return c.tryConnectBlockPayloadRaw(payload, height)
}

func (c *BlockStoreCtx) tryConnectBlockPayloadRaw(raw []byte, height int64) error {
	if c == nil || c.TxIndex == nil || len(raw) < 80 {
		return nil
	}
	// Parallel assist/catch-up workers often race the same height; skip if chainActive is already there.
	if c.Utxo != nil && c.Utxo.TipHeight() >= height {
		return nil
	}
	if !c.hasAncestorBlockBodies(height) {
		return fmt.Errorf("missing ancestor raw blocks through height %d", height-1)
	}
	var chainView consensus.PrevOutView = &consensus.ChainPrevOutView{Index: c.TxIndex, Raw: c.Raw}
	if c.Utxo != nil && c.Utxo.TipHeight() >= height-1 {
		chainView = consensus.MultiPrevOutView{
			consensus.UtxoPrevOutView{Source: c.Utxo},
			&consensus.ChainPrevOutView{Index: c.TxIndex, Raw: c.Raw},
		}
	}
	hdr, err := wire.BlockHeaderFromPayload(raw)
	if err != nil {
		return err
	}
	if err := consensus.ConnectBlockRaw(raw, hdr, height, c.Params.Net, chainView, c.TxIndex, c.Journal); err != nil {
		return err
	}
	// Another worker may have connected this height while we validated.
	if c.Utxo != nil && c.Utxo.TipHeight() >= height {
		return nil
	}
	// Index after connect (Core-style). During deep body IBD defer file-per-txid index so
	// getdata stays ahead of disk; UTXO ApplyBlock still advances chainActive. Index resumes
	// once bodies catch headers (ShouldDeferTxIndexOnPut becomes false).
	if c.Raw != nil && !ShouldDeferTxIndexOnPut(c) {
		c.Raw.IndexStoredBlock(pow.BlockHashLE(raw[:80]), raw)
	}
	if c.shouldUpdateFeeHistoryOnConnect() {
		c.FeeHistory.NotifyBlockHeight(height)
		c.FeeHistory.RecordBlockConnectedRaw(raw, chainView)
		if c.FeeHistoryPath != "" {
			_ = c.FeeHistory.SaveFile(c.FeeHistoryPath)
		}
		if c.FeeEstimatesDatPath != "" {
			_ = c.FeeHistory.SaveCoreFeeEstimatesDat(c.FeeEstimatesDatPath)
		}
	}
	if c.Utxo != nil {
		prevTip := c.Utxo.TipHeight()
		if err := c.Utxo.ApplyBlockRaw(raw, height); err != nil {
			return fmt.Errorf("utxo apply: %w", err)
		}
		if c.Utxo.TipHeight() > prevTip {
			NoteBlockConnected(c.Utxo.TipHeight())
			if c.onChainActiveAdvance != nil {
				c.onChainActiveAdvance(c.Utxo.TipHeight())
			}
		}
	}
	return nil
}

// shouldUpdateFeeHistoryOnConnect skips fee estimator work during deep body IBD (Core defers until near tip).
func (c *BlockStoreCtx) shouldUpdateFeeHistoryOnConnect() bool {
	if c == nil || c.FeeHistory == nil {
		return false
	}
	return !BodiesBehindHeaders(c)
}

// effectiveIBDConnectDeferInterval throttles maybeConnectFrontier during bulk assume-valid IBD.
func effectiveIBDConnectDeferInterval(c *BlockStoreCtx) int64 {
	if c != nil && ShouldPauseHeaderCatchUpForBodyIBD(c, 0) {
		return ibdConnectDeferIntervalDeepIBD
	}
	return ibdConnectDeferInterval
}

func (c *BlockStoreCtx) forwardIBDGap() int64 {
	if c == nil || c.Journal == nil {
		return 0
	}
	tip, err := c.Journal.TipHeight()
	if err != nil || tip < 0 {
		return 0
	}
	cont := c.ContiguousRawHeight()
	if cont < 0 {
		return tip + 1
	}
	if tip <= cont {
		return 0
	}
	return tip - cont
}

// deferConnectDuringIBD skips per-block ConnectBlock while downloading deep history (UTXO catch-up replays later).
func (c *BlockStoreCtx) deferConnectDuringIBD(height int64) bool {
	if c == nil || !BodiesBehindHeaders(c) {
		return false
	}
	// Core always connects genesis-adjacent blocks (coinbase subsidy, maturity, index); do not defer below the forward-IBD window.
	if height < forwardIBDParallelWindow {
		return false
	}
	cont := c.ContiguousRawHeight()
	gap := c.forwardIBDGap()
	// Inv/orphan blocks far ahead of the raw frontier: store only until the gap closes.
	if gap > forwardIBDParallelWindow && height > cont+128 {
		return true
	}
	if c.AssumeValid == nil || !c.AssumeValid.Resolved() {
		return false
	}
	avH := c.AssumeValid.Height()
	if height > avH {
		return false
	}
	tip, _ := c.Journal.TipHeight()
	if tip >= 0 && tip-height < consensus.AssumeValidTipWindow {
		return false
	}
	return true
}

// maybeConnectFrontier runs ConnectTip during IBD only on a throttled schedule in the assume-valid zone.
func (c *BlockStoreCtx) maybeConnectFrontier() {
	if c == nil || c.utxoAheadOfStoredBodies() {
		return
	}
	if ShouldDeferConnectForBodyDownload(c) {
		return
	}
	cont := c.ContiguousRawHeight()
	if c.deferConnectDuringIBD(cont) {
		if cont >= 0 && cont%effectiveIBDConnectDeferInterval(c) != 0 && cont != c.AssumeValid.Height() {
			return
		}
	}
	c.tryConnectContiguousFrontier()
}

// FlushDeferredConnect runs ConnectTip after bulk download (assume-valid zone may have skipped per-batch connect).
// During deep body IBD, use the throttled frontier path so every getdata batch does not kick a full connect storm.
func (c *BlockStoreCtx) FlushDeferredConnect() {
	if c == nil || c.utxoAheadOfStoredBodies() {
		return
	}
	if ShouldDeferConnectForBodyDownload(c) {
		return
	}
	if ShouldPauseHeaderCatchUpForBodyIBD(c, 0) {
		c.maybeConnectFrontier()
		return
	}
	c.tryConnectContiguousFrontier()
}

// tryConnectContiguousFrontier runs ConnectTip from chainActive+1 through the contiguous raw frontier (Core path).
// Uses TryLock so parallel block-assist workers do not pile up redundant ConnectBlock work.
func (c *BlockStoreCtx) tryConnectContiguousFrontier() {
	if c == nil {
		return
	}
	if !utxoConnectMu.TryLock() {
		return
	}
	defer utxoConnectMu.Unlock()
	c.tryConnectContiguousFrontierSteps(connectFrontierMaxSteps(c))
}

// tryConnectContiguousFrontierSteps connects at most maxSteps blocks (0 = no-op).
func (c *BlockStoreCtx) tryConnectContiguousFrontierSteps(maxSteps int) {
	if c == nil || c.Journal == nil || c.Raw == nil || c.TxIndex == nil || maxSteps <= 0 {
		return
	}
	if c.utxoAheadOfStoredBodies() {
		return
	}
	for step := 0; step < maxSteps; step++ {
		cont := c.ContiguousRawHeight()
		next := int64(0)
		if c.Utxo != nil {
			next = c.Utxo.TipHeight() + 1
			if cont >= 0 && c.Utxo.TipHeight() > cont {
				next = cont + 1
			}
		}
		tip, err := c.Journal.TipHeight()
		if err != nil || next > tip {
			return
		}
		if !c.hasAncestorBlockBodies(next) {
			if BodiesBehindHeaders(c) {
				applog.Line("block", fmt.Sprintf("connect height %d: missing ancestor raw bodies", next))
			}
			return
		}
		h80, err := c.Journal.ReadHeaderAt(next)
		if err != nil {
			if BodiesBehindHeaders(c) {
				applog.Line("block", fmt.Sprintf("connect height %d: read header: %v", next, err))
			}
			return
		}
		if !store.HasStoredBodyAtHeight(c.Journal, c.Raw, next, c.chainNet()) {
			if BodiesBehindHeaders(c) {
				applog.Line("block", fmt.Sprintf("connect height %d: raw body missing or undersized on disk", next))
				c.recoverBodiesOnConnectGap(next)
			}
			return
		}
		id := pow.BlockHashLE(h80)
		raw, err := c.Raw.Get(id)
		if err != nil {
			if BodiesBehindHeaders(c) {
				applog.Line("block", fmt.Sprintf("connect height %d: load raw block: %v", next, err))
			}
			if strings.Contains(err.Error(), "hash mismatch") {
				_ = c.Raw.Remove(id)
			}
			c.recoverBodiesOnConnectGap(next)
			return
		}
		if err := c.tryConnectBlockPayloadRaw(raw, next); err != nil {
			if next < 32 || BodiesBehindHeaders(c) {
				applog.Line("block", fmt.Sprintf("connect height %d: %v%s", next, err, consensus.LegacySubsidyBugHint(err)))
			}
			if connectErrNeedsTxIndexRepair(err) {
				maybeRepairTxIndexOnConnectErr(c, err)
				if err = c.tryConnectBlockPayloadRaw(raw, next); err != nil {
					return
				}
			} else {
				return
			}
		}
		c.noteBlockStoredAt(next)
		if ShouldAnnounceConnectedBlocks(c) {
			AnnounceBlockHash(c.announce, id, raw, "")
		}
	}
}

// tryReconnectAround re-runs ConnectBlock for stored blocks near height when bodies caught up (out-of-order fetch).
func (c *BlockStoreCtx) tryReconnectAround(height int64) {
	if c == nil || c.Journal == nil || c.Raw == nil || c.TxIndex == nil {
		return
	}
	if ShouldDeferConnectForBodyDownload(c) {
		return
	}
	tip, err := c.Journal.TipHeight()
	if err != nil {
		return
	}
	lo := height
	if lo > blockReconnectWindow {
		lo = height - blockReconnectWindow
	}
	hi := height + blockReconnectWindow
	if hi > tip {
		hi = tip
	}
	for h := lo; h <= hi; h++ {
		if !c.hasAncestorBlockBodies(h) {
			continue
		}
		h80, err := c.Journal.ReadHeaderAt(h)
		if err != nil {
			continue
		}
		if !store.HasStoredBodyAtHeight(c.Journal, c.Raw, h, c.chainNet()) {
			continue
		}
		id := pow.BlockHashLE(h80)
		raw, err := c.Raw.Get(id)
		if err != nil {
			continue
		}
		if err := c.tryConnectBlockPayloadRaw(raw, h); err != nil {
			applog.Line("block", fmt.Sprintf("reconnect height %d: %v", h, err))
			continue
		}
		c.noteBlockStoredAt(h)
	}
}
