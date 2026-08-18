// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"dogego/analytics"
	"dogego/applog"
	"dogego/chain"
	"dogego/clock"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// BlockStoreCtx groups dependencies for validating and persisting full blocks.
type BlockStoreCtx struct {
	Journal *store.HeaderJournal
	Aux     *store.HeaderAuxJournal
	Params  chain.Params
	Raw     *store.RawBlockStore
	TxIndex *store.TxIndex
	Utxo    *store.UtxoCache
	Policy  *store.ChainPolicy
	// FeeHistory records confirmed block feerates for fee estimation RPC (optional).
	FeeHistory *consensus.FeeHistory
	// FeeHistoryPath persists fee history after ConnectBlock when set (optional).
	FeeHistoryPath string
	// FeeEstimatesDatPath persists Core-shaped TxConfirmStats (optional).
	FeeEstimatesDatPath string

	AssumeValid *consensus.AssumeValid
	// IBDOptimize enables Core-style IBD prioritization (more assist peers, fewer UTXO flushes).
	IBDOptimize bool
	// DbCacheMB is the effective UTXO working-set budget (Core -dbcache).
	DbCacheMB int

	onTipChanged         func(int64)
	onContiguousAdvance  func(contiguous int64)
	onChainActiveAdvance func(height int64)
	// realignBodyDownload runs when ConnectTip needs a raw body below the download cursor.
	realignBodyDownload func(missingHeight int64)
	// OnChainTruncating runs at the start of TruncateChainToHeight (clear in-flight block sync).
	OnChainTruncating func(keepThrough int64)
	// OnChainTruncated runs after TruncateChainToHeight (header rewind / operator truncate).
	OnChainTruncated func(keepThrough int64)
	announce         BlockAnnounceEnv
	forkProbe        ForkProbeFunc
	chainElection    ChainElectionFunc

	contiguousMu  sync.Mutex
	contiguousTip int64 // highest height h with raw bodies for [0,h]; -1 = not initialized

	// OnBlockFromPeer is called when a P2P peer delivers a stored block (getdata or broadcast).
	OnBlockFromPeer func(peerAddr string, blockHeight int64)
	// NetworkUnix returns Core GetTime for header/block time checks (median peer offset when wired).
	NetworkUnix func() int64
	// ChainWork caches cumulative chain work for fast IBD / dashboard queries (optional).
	ChainWork *ChainWorkCache
	// Analytics is the shared Pebble sidecar (optional); used to record reorg events before prune.
	Analytics *analytics.DB
	// NetworkSlug is mainnet / testnet / etc. for reorg analytics rows.
	NetworkSlug string
	// lastBadNBitsRewind is the last bad-nBits rewind height (-1 = none); enables stepping back further.
	lastBadNBitsRewind int64
	// badNBitsRepeatHeight/Count track repeated bad-nBits retries at the same tip to force peer rotation.
	badNBitsRepeatHeight int64
	badNBitsRepeatCount  int
}

// SetBodyDownloadRealign wires IBD cursor realignment when connect needs a missing body below nextProbe.
func (c *BlockStoreCtx) SetBodyDownloadRealign(fn func(missingHeight int64)) {
	if c != nil {
		c.realignBodyDownload = fn
	}
}

// maybeRealignBodyDownloadOnConnectGap nudges block download toward the next body ConnectTip needs.
func (c *BlockStoreCtx) maybeRealignBodyDownloadOnConnectGap() {
	if c == nil || c.realignBodyDownload == nil {
		return
	}
	if gap := ConnectBodyGapHeight(c); gap >= 0 {
		c.realignBodyDownload(gap)
	}
}

func (c *BlockStoreCtx) realignBodyDownloadForConnect(missingHeight int64) {
	if c == nil || missingHeight < 0 || c.realignBodyDownload == nil {
		return
	}
	c.realignBodyDownload(missingHeight)
}

func (c *BlockStoreCtx) NetworkTimeUnix() int64 {
	if c != nil && c.NetworkUnix != nil {
		return c.NetworkUnix()
	}
	return clock.UnixNow()
}

// SetNetworkTimeSource wires Core GetTime (median peer offset, else handshake peer nTime).
func (c *BlockStoreCtx) SetNetworkTimeSource(peerMgr *PeerMgr, handshakePeer *wire.DecodedVersion) {
	if c == nil {
		return
	}
	c.NetworkUnix = func() int64 {
		return wireNetworkUnix(peerMgr, handshakePeer)
	}
}

// SetBlockAnnounce configures inv relay after ConnectBlock when near header tip (Core).
func (c *BlockStoreCtx) SetBlockAnnounce(env BlockAnnounceEnv) {
	if c != nil {
		c.announce = env
	}
}

// AnnounceConnectedBlock relays a stored block using cmpctblock (HB peers) or inv (others).
func (c *BlockStoreCtx) AnnounceConnectedBlock(raw []byte) {
	if c == nil || len(raw) < 80 {
		return
	}
	hash := pow.BlockHashLE(raw[:80])
	AnnounceBlockHash(c.announce, hash, raw, "")
}

// SetPrimaryCmpctHBTo records whether the primary sync peer receives high-bandwidth cmpctblock.
func (c *BlockStoreCtx) SetPrimaryCmpctHBTo(v bool) {
	if c != nil {
		c.announce.PrimaryCmpctHBTo = v
	}
}

// SetForkProbe configures relay getheaders at a fork height before header reorg truncate.
func (c *BlockStoreCtx) SetForkProbe(fn ForkProbeFunc) {
	if c != nil {
		c.forkProbe = fn
	}
}

// SetChainElection configures synchronous multi-peer chain-work comparison before reorg truncate.
func (c *BlockStoreCtx) SetChainElection(fn ChainElectionFunc) {
	if c != nil {
		c.chainElection = fn
	}
}

// NewBlockStoreCtx returns a block store context with contiguous-body tracking initialized.
func NewBlockStoreCtx(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, raw *store.RawBlockStore, txIx *store.TxIndex, utxo *store.UtxoCache) *BlockStoreCtx {
	return &BlockStoreCtx{
		Journal:              j,
		Aux:                  aux,
		Params:               p,
		Raw:                  raw,
		TxIndex:              txIx,
		Utxo:                 utxo,
		contiguousTip:        -1,
		lastBadNBitsRewind:   -1,
		badNBitsRepeatHeight: -1,
	}
}

const rpcSyncUtxoMaxBlocks = 8

// SyncUtxoCacheBounded advances chainActive by at most maxBlocks (RPC/wallet paths; avoids blocking the server during deep IBD).
func (c *BlockStoreCtx) SyncUtxoCacheBounded(maxBlocks int) error {
	if c == nil || maxBlocks <= 0 {
		return c.SyncUtxoCache()
	}
	var err error
	withUtxoConnectLock(func() error {
		err = c.syncUtxoCacheBoundedLocked(maxBlocks)
		return err
	})
	return err
}

func (c *BlockStoreCtx) syncUtxoCacheBoundedLocked(maxBlocks int) error {
	if c.Utxo == nil || c.Journal == nil || c.Raw == nil || c.TxIndex == nil {
		return nil
	}
	if c.utxoAheadOfStoredBodies() {
		return nil
	}
	c.contiguousMu.Lock()
	if c.contiguousTip < 0 {
		c.recomputeContiguousTipLocked()
	} else {
		c.extendContiguousForwardLocked()
	}
	to := c.contiguousTip
	c.contiguousMu.Unlock()
	if to < 0 {
		return nil
	}
	before := c.Utxo.TipHeight()
	steps := maxBlocks
	if cap := connectFrontierMaxSteps(c); steps > cap {
		steps = cap
	}
	c.tryConnectContiguousFrontierSteps(steps)
	after := c.Utxo.TipHeight()
	if after > before {
		return nil
	}
	if after >= to {
		return nil
	}
	return fmt.Errorf("utxo sync: connect stalled at height %d (contiguous bodies through %d)", after, to)
}

// connectUtxoLockChunkSize splits IBD connect batches so JSON-RPC can interleave between chunks.
func connectUtxoLockChunkSize(total int, bs *BlockStoreCtx) int {
	chunk := 64
	if bs != nil && !connectFrontierScriptsEnabled(bs) {
		chunk = 128
	}
	if total <= chunk {
		return total
	}
	return chunk
}

// SyncUtxoCache extends the UTXO set through the highest contiguous stored height.
// During body IBD, work is capped per call so RPC and the connect worker stay responsive.
func (c *BlockStoreCtx) SyncUtxoCache() error {
	if c != nil && BodiesBehindHeaders(c) {
		budget := connectCatchUpBlocksPerIBDCall(c)
		chunk := connectUtxoLockChunkSize(budget, c)
		for spent := 0; spent < budget; {
			steps := chunk
			if rem := budget - spent; rem < steps {
				steps = rem
			}
			var err error
			if e := withUtxoConnectLock(func() error {
				err = c.syncUtxoCacheBoundedLocked(steps)
				return err
			}); e != nil {
				return e
			}
			if err != nil {
				return err
			}
			spent += steps
		}
		return nil
	}
	return withUtxoConnectLock(func() error {
		return c.syncUtxoCacheFull()
	})
}

func (c *BlockStoreCtx) syncUtxoCacheFull() error {
	if c == nil || c.Utxo == nil || c.Journal == nil || c.Raw == nil || c.TxIndex == nil {
		return nil
	}
	if c.utxoAheadOfStoredBodies() {
		return nil
	}
	c.contiguousMu.Lock()
	if c.contiguousTip < 0 {
		c.recomputeContiguousTipLocked()
	} else {
		c.extendContiguousForwardLocked()
	}
	to := c.contiguousTip
	c.contiguousMu.Unlock()
	if to < 0 {
		return nil
	}
	maxConnectPasses := syncUtxoMaxConnectPasses(c, to)
	for pass := 0; pass < maxConnectPasses; pass++ {
		before := c.Utxo.TipHeight()
		c.tryConnectContiguousFrontierSteps(connectFrontierMaxSteps(c))
		after := c.Utxo.TipHeight()
		if after >= to || after == before {
			break
		}
	}
	if c.Utxo.TipHeight() >= to {
		return nil
	}
	stallErr := fmt.Errorf("utxo sync: connect stalled at height %d (contiguous bodies through %d)", c.Utxo.TipHeight(), to)
	maybeRepairTxIndexOnConnectStall(c, stallErr)
	if isConnectStallErr(stallErr) {
		before := c.Utxo.TipHeight()
		for pass := 0; pass < maxConnectPasses; pass++ {
			c.tryConnectContiguousFrontierSteps(connectFrontierMaxSteps(c))
			after := c.Utxo.TipHeight()
			if after >= to || after == before {
				break
			}
			before = after
		}
	}
	if c.Utxo.TipHeight() >= to {
		return nil
	}
	return stallErr
}

// RebuildUtxoThrough resets the UTXO cache and replays ConnectBlock for heights [0, through] (Core chainstate replay).
func (c *BlockStoreCtx) RebuildUtxoThrough(through int64) error {
	if c == nil || c.Utxo == nil || c.Journal == nil || c.Raw == nil || c.TxIndex == nil {
		return nil
	}
	if through < 0 {
		return fmt.Errorf("utxo rebuild: negative through %d", through)
	}
	c.Utxo.Reset()
	c.contiguousMu.Lock()
	if c.contiguousTip < through {
		c.recomputeContiguousTipLocked()
	}
	c.contiguousMu.Unlock()
	for h := int64(0); h <= through; h++ {
		if !c.hasAncestorBlockBodies(h) {
			return fmt.Errorf("utxo rebuild: missing raw block at height %d", h)
		}
		h80, err := c.Journal.ReadHeaderAt(h)
		if err != nil {
			return err
		}
		if !store.HasStoredBodyAtHeight(c.Journal, c.Raw, h, c.chainNet()) {
			return fmt.Errorf("utxo rebuild: missing raw block at height %d", h)
		}
		id := pow.BlockHashLE(h80)
		raw, err := c.Raw.Get(id)
		if err != nil {
			return err
		}
		if err := c.tryConnectBlockPayloadRaw(raw, h); err != nil {
			return fmt.Errorf("height %d connect: %w", h, err)
		}
	}
	return nil
}

func (c *BlockStoreCtx) chainNet() chain.Network {
	if c == nil {
		return 0
	}
	return c.Params.Net
}

// PurgeStaleRawBlockTemps removes incomplete *.bin.tmp files left by interrupted atomic Puts.
func (c *BlockStoreCtx) PurgeStaleRawBlockTemps() (int, error) {
	if c == nil || c.Raw == nil {
		return 0, nil
	}
	return c.Raw.PurgeStaleRawBlockTemps()
}

// PurgeInadequateRawBodiesThroughHeight removes unreadable bodies for heights [0, through].
func (c *BlockStoreCtx) PurgeInadequateRawBodiesThroughHeight(through int64) (int, error) {
	if c == nil || c.Raw == nil {
		return 0, nil
	}
	n, lowest, err := store.PurgeInadequateRawBodiesThroughHeight(c.Journal, c.Raw, through, c.chainNet())
	if err != nil {
		return n, err
	}
	if n > 0 {
		c.afterRawBodiesPurged(lowest)
	}
	return n, nil
}

// PurgeInadequateRawBodies removes undersized raw block files and resets contiguous coverage.
func (c *BlockStoreCtx) PurgeInadequateRawBodies() (int, error) {
	if c == nil || c.Raw == nil {
		return 0, nil
	}
	n, lowest, err := store.PurgeInadequateRawBodies(c.Journal, c.Raw, c.chainNet())
	if err != nil {
		return n, err
	}
	if n > 0 {
		c.afterRawBodiesPurged(lowest)
	}
	return n, nil
}

func (c *BlockStoreCtx) afterRawBodiesPurged(lowestRemoved int64) {
	if c == nil {
		return
	}
	if lowestRemoved >= 0 && c.utxoAheadOfStoredBodies() {
		prev := c.ContiguousRawHeight()
		if lowestRemoved <= prev+1 {
			c.shrinkContiguousTipAfterBodyRemoved(lowestRemoved)
			if after := c.ContiguousRawHeight(); prev >= 0 && after >= 0 && after < prev {
				applog.Line("block", fmt.Sprintf("replay purge: contiguous %d → %d (removed from height %d)", prev, after, lowestRemoved))
			}
		} else if prev >= 0 {
			applog.Line("block", fmt.Sprintf("replay purge: removed %d body(s) below frontier (contiguous %d unchanged)", 1, prev))
		}
		return
	}
	c.ResetContiguousTip()
}

// recoverBodiesOnConnectGap purges the unreadable body at next (if any), refreshes contiguous
// coverage, and realigns block download to the connect frontier.
func (c *BlockStoreCtx) recoverBodiesOnConnectGap(next int64) {
	if c == nil || next < 0 {
		return
	}
	if c.utxoAheadOfStoredBodies() {
		if cont := c.ContiguousRawHeight(); cont >= 0 && next < cont {
			return
		}
	}
	prev := c.ContiguousRawHeight()
	if removed, err := c.purgeUnreadableBodyAtHeight(next); err != nil {
		applog.Line("block", "connect gap purge: "+err.Error())
	} else if removed {
		applog.Line("block", fmt.Sprintf("connect gap at height %d: removed unreadable raw block", next))
		if c.utxoAheadOfStoredBodiesAt(prev) {
			c.shrinkContiguousTipAfterBodyRemoved(next)
		} else {
			refreshed := c.RevalidateContiguousTip()
			if prev >= 0 && refreshed >= 0 && refreshed < prev {
				applog.Line("block", fmt.Sprintf("connect gap: contiguous %d → %d after purge/refresh", prev, refreshed))
			}
		}
	}
	c.realignBodyDownloadForConnect(next)
}

// recoverBodiesOnConnectGapFull scans the raw store for unreadable bodies (bundled locators,
// stubs, hash mismatches) and realigns download - used on IBD stall recovery, not per connect miss.
func (c *BlockStoreCtx) recoverBodiesOnConnectGapFull(next int64) {
	if c == nil || next < 0 {
		return
	}
	prev := c.ContiguousRawHeight()
	if n, err := c.PurgeInadequateRawBodies(); err != nil {
		applog.Line("block", "connect gap full purge: "+err.Error())
	} else if n > 0 {
		applog.Line("block", fmt.Sprintf("connect gap at height %d: removed %d unreadable raw block(s)", next, n))
	}
	refreshed := c.RevalidateContiguousTip()
	if prev >= 0 && refreshed >= 0 && refreshed < prev {
		applog.Line("block", fmt.Sprintf("connect gap: contiguous %d → %d after purge/refresh", prev, refreshed))
	}
	c.realignBodyDownloadForConnect(next)
}

func (c *BlockStoreCtx) purgeUnreadableBodyAtHeight(height int64) (bool, error) {
	if c == nil || c.Journal == nil || c.Raw == nil || height < 0 {
		return false, nil
	}
	if store.HasStoredBodyAtHeight(c.Journal, c.Raw, height, c.chainNet()) {
		return false, nil
	}
	h80, err := c.Journal.ReadHeaderAt(height)
	if err != nil {
		return false, err
	}
	hashLE := pow.BlockHashLE(h80)
	if !c.Raw.Has(hashLE) {
		return false, nil
	}
	if err := c.Raw.Remove(hashLE); err != nil {
		return false, err
	}
	return true, nil
}

// rawBodyPresent reports whether a full block payload is on disk (ignores undersized test stubs).
func (c *BlockStoreCtx) rawBodyPresent(hash [32]byte, height int64) bool {
	if c == nil || c.Raw == nil {
		return false
	}
	return c.Raw.HasStoredBody(hash, store.MinRawBlockBytes(c.chainNet(), height))
}

// StoreValidatedBlock checks consensus rules when the header is in the journal, then writes rawblocks/.
// Skips work when the block id is already stored. Wire-level checks always run before Put.
func (c *BlockStoreCtx) StoreValidatedBlock(want [32]byte, payload []byte) error {
	return c.storeValidatedBlock(want, payload, -1, false)
}

// StoreValidatedBlockAtHeight is like StoreValidatedBlock but skips a journal hash scan when height is known.
func (c *BlockStoreCtx) StoreValidatedBlockAtHeight(want [32]byte, payload []byte, height int64) error {
	return c.storeValidatedBlock(want, payload, height, height >= 0)
}

func (c *BlockStoreCtx) storeValidatedBlock(want [32]byte, payload []byte, knownHeight int64, trustHeight bool) error {
	if c == nil || c.Raw == nil || len(payload) < 81 {
		return fmt.Errorf("block store: missing store or payload")
	}
	if err := wire.ValidateBlockPayload(payload, want); err != nil {
		return err
	}
	height := knownHeight
	if height < 0 && c.Journal != nil {
		display := pow.BlockHashHex(payload[:80])
		h, herr := c.Journal.HeightByDisplayHash(display)
		if herr != nil {
			var extendErr error
			height, extendErr = c.tryExtendChainFromPayload(payload, c.NetworkTimeUnix())
			if extendErr != nil {
				return fmt.Errorf("block not in header chain (%s…): %w", display[:12], extendErr)
			}
		} else {
			height = h
		}
	} else if height >= 0 && c.Journal != nil && !trustHeight {
		hJournal, err := c.Journal.ReadHeaderAt(height)
		if err != nil || len(hJournal) != 80 {
			return fmt.Errorf("journal read height %d: %w", height, err)
		}
		if !bytes.Equal(hJournal, payload[:80]) {
			return fmt.Errorf("block header != journal at height %d", height)
		}
	}
	if c.rawBodyPresent(want, height) {
		return nil
	}
	if height >= 0 {
		if minB := store.MinRawBlockBytes(c.chainNet(), height); minB > 0 && len(payload) < minB {
			return fmt.Errorf("raw block too short at height %d: %d bytes (need >= %d)", height, len(payload), minB)
		}
	}
	if height >= 0 && !(trustHeight && ShouldDeferConnectForBodyDownload(c)) {
		if err := consensus.CheckBlockPayload(payload, want, height, c.Params.Net); err != nil {
			return fmt.Errorf("block consensus: %w", err)
		}
		if !trustHeight {
			var chainView consensus.PrevOutView
			if c.Utxo != nil && c.Utxo.TipHeight() >= height-1 {
				chainView = consensus.UtxoPrevOutView{Source: c.Utxo}
			} else if c.TxIndex != nil && c.Raw != nil {
				chainView = &consensus.ChainPrevOutView{Index: c.TxIndex, Raw: c.Raw}
			}
			if err := consensus.CheckBlockCoinbaseSubsidyPayload(payload, height, c.Params.Net, chainView); err != nil {
				if hint := consensus.LegacySubsidyBugHint(err); hint != "" {
					return fmt.Errorf("block consensus: %w%s", err, hint)
				}
				return fmt.Errorf("block consensus: %w", err)
			}
		}
	}
	// Core AcceptBlock persists block data before ConnectTip; do not drop valid downloaded bodies when
	// ConnectBlock fails transiently (UTXO/index lag). maybeConnectFrontier replays connect.
	if err := c.Raw.Put(want, payload); err != nil {
		return err
	}
	if height >= 0 {
		if trustHeight {
			c.noteBlockStoredAtDeferred(height)
		} else {
			c.noteBlockStoredAt(height)
		}
		logStoredBlockSampled(c, height, want)
		NoteBlockStored(height)
		if !trustHeight {
			if !c.deferConnectDuringIBD(height) {
				if err := c.tryConnectBlockPayloadRaw(payload, height); err != nil {
					hint := consensus.LegacySubsidyBugHint(err)
					if !c.hasAncestorBlockBodies(height) {
						applog.Line("block", fmt.Sprintf("defer connect at height %d (%v)", height, err))
					} else if hint != "" {
						return fmt.Errorf("block connect: %w%s", err, hint)
					} else {
						applog.Line("block", fmt.Sprintf("connect height %d pending (%v)", height, err))
					}
				}
			}
			c.maybeConnectFrontier()
			c.tryReconnectAround(height)
		}
	}
	return nil
}

var (
	storedBlockLogMu   sync.Mutex
	storedBlockLogAt   time.Time
	storedBlockLogLast int64
)

// logStoredBlockSampled avoids per-block applog during IBD (UI console + lock churn).
func logStoredBlockSampled(c *BlockStoreCtx, height int64, want [32]byte) {
	if BodiesBehindHeaders(c) {
		now := time.Now()
		storedBlockLogMu.Lock()
		logIt := height%512 == 0 || now.Sub(storedBlockLogAt) >= 2*time.Second || height < storedBlockLogLast
		if logIt {
			storedBlockLogAt = now
			storedBlockLogLast = height
		}
		storedBlockLogMu.Unlock()
		if !logIt {
			return
		}
	}
	applog.Line("block", fmt.Sprintf("stored block height %d (%x…)", height, want[:4]))
}
