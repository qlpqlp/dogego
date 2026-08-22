// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// rawBatchClaim is a claimed contiguous run of missing block heights (in flight until finishBatch).
type rawBatchClaim struct {
	heights []int64
	hashes  [][32]byte
	lo, hi  int64
}

// progressiveRawState coordinates forward raw-block download with parallel workers via in-flight claims.
type progressiveRawState struct {
	mu           sync.Mutex
	chainDir     string
	lastTip      int64
	nextProbe    int64 // scan start for gaps (ascending)
	idleFull     bool
	diskPressurePaused bool // low datadir free space; claimBatch refuses new body getdata
	inFlight     map[int64][32]byte
	inFlightLane map[int64]int // height → sync lane (0 = primary)
	syncWorkers  int           // parallel download lanes (primary + block-assist); 1 = no striping

	ibdStarted      time.Time
	blocksStoredIBD int64
	lastStoredAt    time.Time
	rateSamples     []ibdRateSample // recent window for stable blk/min (not lifetime mean)

	laneAddr          map[int]string // sync lane → peer host:port (block stall detection)
	peerLane          map[string]int // peer host:port → unique getdata lane (no hash collisions)
	stallingSince     time.Time      // Core nStallingSince when frontier height is in-flight
	softStallFrontier int64          // last frontier soft-released (-1 none); next stall hard-disconnects
	lastStallPeer     string         // last peer penalized for block stalling (RPC snapshot)
	lastStallAt       time.Time

	laneDownloadSince       map[int]time.Time // per sync lane: first getdata in current batch
	lastDownloadTimeoutPeer string
	lastDownloadTimeoutAt   time.Time

	laneDelivery map[int][]laneDeliverySample // recent deliveries for adaptive in-flight budgets

	contiguousCheckpoint int64           // persisted monotonic raw coverage; -2 = not loaded
	contigRateSamples    []ibdRateSample // contiguous tip samples for hole-fill blk/min

	activeBatch map[int]*batchSlot // lane → in-progress getdata cancel (header rewind abort)
	batchGen    int
}

type batchSlot struct {
	gen    int
	cancel context.CancelFunc
}

// ibdRateSample tracks cumulative blocks stored for a recent-window download rate.
type ibdRateSample struct {
	at  time.Time
	cum int64
}

const (
	ibdRateWindow    = 10 * time.Minute
	ibdRateMinWindow = 2 * time.Second
)

func (s *progressiveRawState) syncWorkerCount() int {
	if s == nil {
		return 1
	}
	s.mu.Lock()
	n := s.syncWorkers
	s.mu.Unlock()
	if n < 1 {
		return 1
	}
	return n
}

// laneForAddr picks a unique sync lane for a non-primary P2P link (lane 0 is reserved for the primary peer).
// Relays use a bounded pool above assist IDs. Growing syncWorkers per relay (live: 46 lanes)
// inflated download timeouts to 1h and leaked thousands of in-flight claims.
func (s *progressiveRawState) laneForAddr(addr string) int {
	if s == nil || addr == "" {
		return 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.peerLane == nil {
		s.peerLane = make(map[string]int)
	}
	if id, ok := s.peerLane[addr]; ok && id > 0 {
		return id
	}
	used := make(map[int]struct{}, len(s.peerLane)+len(s.activeBatch)+len(s.laneAddr))
	for _, id := range s.peerLane {
		used[id] = struct{}{}
	}
	for lane := range s.activeBatch {
		used[lane] = struct{}{}
	}
	for lane := range s.laneAddr {
		used[lane] = struct{}{}
	}
	// Lane 0 is primary; 1..syncWorkers-1 are block-assist workers. Relays must not
	// reuse those IDs or ReleaseLaneInFlight on assist disconnect cancels relay getdata.
	start := 1
	if s.syncWorkers > 1 {
		start = s.syncWorkers
	}
	limit := start + defaultMaxOutbound
	id := -1
	for cand := start; cand < limit; cand++ {
		if _, taken := used[cand]; !taken {
			id = cand
			break
		}
	}
	if id < 0 {
		return -1
	}
	s.peerLane[addr] = id
	return id
}

func (s *progressiveRawState) peekLaneForAddr(addr string) int {
	if s == nil || addr == "" {
		return -1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.peerLane == nil {
		return -1
	}
	id, ok := s.peerLane[addr]
	if !ok || id <= 0 {
		return -1
	}
	return id
}

// SetSyncParallelism configures height stripes for parallel getdata (include primary link in the count).
func (s *progressiveRawState) SetSyncParallelism(workers int) {
	if s == nil || workers < 1 {
		return
	}
	if workers > maxBlockAssistWorkers+1 {
		workers = maxBlockAssistWorkers + 1
	}
	s.mu.Lock()
	s.syncWorkers = workers
	s.mu.Unlock()
}

// PrepareAtStartup arms block download when headers.bin already has a chain but rawblocks/ lag.
// Call before P2P connect so assist workers and the main loop treat body sync as active immediately.
func (s *progressiveRawState) PrepareAtStartup(bs *BlockStoreCtx) {
	if s == nil || bs == nil || bs.Journal == nil || bs.Raw == nil {
		return
	}
	_ = EnsureLocalGenesis(bs)
	j := bs.Journal
	rs := bs.Raw
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return
	}
	cont := bs.ContiguousRawHeight()
	if j != nil {
		j.ReconcileCountCacheFromDisk()
	}
	low, err := LowestMissingForIBD(j, rs, cont, tip, bs)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight == nil {
		s.inFlight = make(map[int64][32]byte)
	}
	if s.inFlightLane == nil {
		s.inFlightLane = make(map[int64]int)
	}
	if s.laneAddr == nil {
		s.laneAddr = make(map[int]string)
	}
	s.onTipChangedLocked(tip)
	if err != nil || low < 0 {
		s.idleFull = true
		if cont >= 0 {
			s.nextProbe = cont + 1
		} else {
			s.nextProbe = 0
		}
		return
	}
	s.idleFull = false
	// Stale rawblocks_sync.json can point above the contiguous frontier (e.g. resume at 667 while
	// heights 1-666 were never connected). Always fill from the lowest missing height first.
	if low >= 0 && (s.nextProbe < low || s.nextProbe > tip || (cont >= 0 && s.nextProbe > cont+1) || s.nextProbe > low) {
		s.nextProbe = low
	}
	if s.nextProbe < 0 {
		s.nextProbe = 0
	}
	if s.ibdStarted.IsZero() {
		s.ibdStarted = time.Now()
	}
	applog.Line("block", fmt.Sprintf("local headers at height %d (bodies through %d); block sync active from height %d", tip, cont, s.nextProbe))
}

func (s *progressiveRawState) noteBlocksStoredLocked(n int) {
	if s == nil || n <= 0 {
		return
	}
	if s.ibdStarted.IsZero() {
		s.ibdStarted = time.Now()
	}
	s.blocksStoredIBD += int64(n)
	s.lastStoredAt = time.Now()
	s.rateSamples = append(s.rateSamples, ibdRateSample{at: s.lastStoredAt, cum: s.blocksStoredIBD})
	cutoff := s.lastStoredAt.Add(-ibdRateWindow)
	i := 0
	for i < len(s.rateSamples) && s.rateSamples[i].at.Before(cutoff) {
		i++
	}
	if i > 0 {
		s.rateSamples = append([]ibdRateSample(nil), s.rateSamples[i:]...)
	}
}

// recentBlocksPerMinuteLocked returns blk/min over the last ~10 minutes (0 if window too short).
func (s *progressiveRawState) recentBlocksPerMinuteLocked() float64 {
	if s == nil || len(s.rateSamples) < 2 {
		return 0
	}
	first, last := s.rateSamples[0], s.rateSamples[len(s.rateSamples)-1]
	elapsed := last.at.Sub(first.at)
	if elapsed < ibdRateMinWindow {
		return 0
	}
	delta := last.cum - first.cum
	if delta <= 0 {
		return 0
	}
	return float64(delta) / elapsed.Minutes()
}

// noteContiguousTipLocked records contiguous tip for hole-fill rate (dashboard ETA).
func (s *progressiveRawState) noteContiguousTipLocked(cont int64) {
	if s == nil || cont < 0 {
		return
	}
	now := time.Now()
	if n := len(s.contigRateSamples); n > 0 && s.contigRateSamples[n-1].cum == cont {
		return
	}
	s.contigRateSamples = append(s.contigRateSamples, ibdRateSample{at: now, cum: cont})
	cutoff := now.Add(-ibdRateWindow)
	i := 0
	for i < len(s.contigRateSamples) && s.contigRateSamples[i].at.Before(cutoff) {
		i++
	}
	if i > 0 {
		s.contigRateSamples = append([]ibdRateSample(nil), s.contigRateSamples[i:]...)
	}
}

// recentContiguousBlocksPerMinuteLocked is hole-fill blk/min (what operators mean by "stored").
func (s *progressiveRawState) recentContiguousBlocksPerMinuteLocked() float64 {
	if s == nil || len(s.contigRateSamples) < 2 {
		return 0
	}
	first, last := s.contigRateSamples[0], s.contigRateSamples[len(s.contigRateSamples)-1]
	elapsed := last.at.Sub(first.at)
	if elapsed < ibdRateMinWindow {
		return 0
	}
	delta := last.cum - first.cum
	if delta <= 0 {
		return 0
	}
	return float64(delta) / elapsed.Minutes()
}

// initProgressiveRawAtStartup restores checkpoint, arms sync lanes, and realigns the download probe.
func (s *progressiveRawState) initProgressiveRawAtStartup(chainDir string, bs *BlockStoreCtx, syncLanes int) {
	if s == nil || bs == nil {
		return
	}
	tip, _ := bs.Journal.TipHeight()
	cont := bs.ContiguousRawHeight()
	s.InitFromCheckpoint(chainDir, tip, cont)
	if bs != nil && s.contiguousCheckpoint >= 0 {
		bs.TrySeedContiguousFromCheckpoint(s.contiguousCheckpoint)
	}
	if syncLanes > 0 {
		s.SetSyncParallelism(syncLanes)
	}
	s.PrepareAtStartup(bs)
	if cont >= 0 {
		s.SyncCheckpointToContiguous(cont)
	}
}

// InitFromCheckpoint restores nextProbe from rawblocks_sync.json when present.
// contiguous is the connected raw-body height (-1 if unknown); stale checkpoints far ahead of
// bodies are clamped so forward IBD always resumes from the lowest missing height.
func (s *progressiveRawState) InitFromCheckpoint(chainDir string, tip, contiguous int64) {
	s.chainDir = chainDir
	s.softStallFrontier = -1
	if s.inFlight == nil {
		s.inFlight = make(map[int64][32]byte)
	}
	if s.inFlightLane == nil {
		s.inFlightLane = make(map[int64]int)
	}
	if chainDir == "" || tip < 0 {
		return
	}
	cp, err := store.LoadRawBlockSyncCheckpoint(chainDir)
	if err != nil {
		applog.Line("block", "rawblocks_sync checkpoint read: "+err.Error())
		return
	}
	s.contiguousCheckpoint = cp.ContiguousRawHeight
	if cp.NextProbeHeight < 0 || cp.NextProbeHeight > tip {
		return
	}
	probe := cp.NextProbeHeight
	if contiguous >= 0 && probe > contiguous+1 {
		old := probe
		probe = contiguous + 1
		if probe < 0 {
			probe = 0
		}
		applog.Line("block", fmt.Sprintf("rawblocks_sync checkpoint height %d ahead of contiguous %d; forward sync from %d (tip %d)", old, contiguous, probe, tip))
	} else if contiguous < 0 && probe > 0 {
		old := probe
		probe = 0
		applog.Line("block", fmt.Sprintf("rawblocks_sync checkpoint height %d with no contiguous bodies; forward sync from genesis (tip %d)", old, tip))
	} else {
		applog.Line("block", fmt.Sprintf("resuming raw block sync from height %d (tip %d)", probe, tip))
	}
	s.nextProbe = probe
}

func (s *progressiveRawState) persistCheckpointLocked() {
	if s.chainDir == "" {
		return
	}
	h := s.nextProbe
	if h < 0 {
		h = 0
	}
	cp := store.RawBlockSyncCheckpoint{NextProbeHeight: h}
	if s.contiguousCheckpoint >= -1 {
		cp.ContiguousRawHeight = s.contiguousCheckpoint
	}
	if err := store.SaveRawBlockSyncCheckpoint(s.chainDir, cp); err != nil {
		applog.Line("block", "rawblocks_sync checkpoint write: "+err.Error())
	}
}

// ReleaseLaneInFlight drops active getdata claims for one sync lane (primary disconnect / assist session end).
func (s *progressiveRawState) ReleaseLaneInFlight(lane int) int {
	if s == nil || lane < 0 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var freed int
	for h, w := range s.inFlightLane {
		if w != lane {
			continue
		}
		delete(s.inFlight, h)
		delete(s.inFlightLane, h)
		freed++
	}
	if slot := s.activeBatch[lane]; slot != nil && slot.cancel != nil {
		slot.cancel()
		delete(s.activeBatch, lane)
	}
	if s.laneAddr != nil {
		if addr := s.laneAddr[lane]; addr != "" && s.peerLane != nil {
			delete(s.peerLane, addr)
		}
		delete(s.laneAddr, lane)
	}
	if freed > 0 {
		s.idleFull = false
		applog.Line("block", fmt.Sprintf("released %d in-flight block height(s) on sync lane %d (peer disconnected)", freed, lane))
	}
	delete(s.laneDownloadSince, lane)
	return freed
}

// releaseOrphanInFlight drops getdata claims whose lane has no live peer (relay/assist
// disconnect used to leak thousands of heights and freeze the contiguous hole).
func (s *progressiveRawState) releaseOrphanInFlight() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.releaseOrphanInFlightLocked()
}

func (s *progressiveRawState) releaseOrphanInFlightLocked() int {
	if len(s.inFlight) == 0 && len(s.inFlightLane) == 0 {
		return 0
	}
	live := make(map[int]struct{}, len(s.activeBatch)+len(s.laneAddr))
	for lane := range s.activeBatch {
		live[lane] = struct{}{}
	}
	for lane := range s.laneAddr {
		live[lane] = struct{}{}
	}
	if len(live) == 0 {
		n := len(s.inFlight)
		s.inFlight = make(map[int64][32]byte)
		s.inFlightLane = make(map[int64]int)
		if n > 0 {
			s.idleFull = false
			applog.Line("block", fmt.Sprintf("released %d orphan in-flight height(s) (no live download peer)", n))
		}
		return n
	}
	freed := 0
	for h, lane := range s.inFlightLane {
		if _, ok := live[lane]; ok {
			continue
		}
		delete(s.inFlight, h)
		delete(s.inFlightLane, h)
		freed++
	}
	for h := range s.inFlight {
		if _, ok := s.inFlightLane[h]; ok {
			continue
		}
		delete(s.inFlight, h)
		freed++
	}
	if freed > 0 {
		s.idleFull = false
		applog.Line("block", fmt.Sprintf("released %d orphan in-flight height(s) (download peer gone)", freed))
	}
	return freed
}

// releaseStaleInFlightBelowContiguousLocked drops getdata claims at or below the contiguous tip.
// Those heights are already covered (or unreachable as the download frontier) and otherwise
// poison claimBatch into treating min(inFlight) as the hole.
func (s *progressiveRawState) releaseStaleInFlightBelowContiguousLocked(cont int64) int {
	if s == nil || len(s.inFlight) == 0 {
		return 0
	}
	frontier := cont + 1
	if cont < 0 {
		frontier = 0
	}
	freed := 0
	for h := range s.inFlight {
		if h >= frontier {
			continue
		}
		delete(s.inFlight, h)
		delete(s.inFlightLane, h)
		freed++
	}
	if freed > 0 {
		s.idleFull = false
	}
	return freed
}

// ResetInFlightForHeaderRewind clears in-flight getdata claims during header journal truncate.
// Does not rescan the journal (PrepareAtStartup) so truncate is not blocked on progressiveRawState.mu.
func (s *progressiveRawState) ResetInFlightForHeaderRewind() {
	if s == nil {
		return
	}
	s.cancelAllActiveBatches()
	s.mu.Lock()
	s.inFlight = make(map[int64][32]byte)
	s.inFlightLane = make(map[int64]int)
	s.laneAddr = make(map[int]string)
	s.peerLane = make(map[string]int)
	s.laneDownloadSince = make(map[int]time.Time)
	s.stallingSince = time.Time{}
	s.idleFull = false
	s.mu.Unlock()
}

// ResetAfterChainTruncate clears in-flight claims and re-arms forward sync from the lowest missing height.
func (s *progressiveRawState) ResetAfterChainTruncate(bs *BlockStoreCtx) {
	if s == nil || bs == nil {
		return
	}
	s.ResetInFlightForHeaderRewind()
	s.PrepareAtStartup(bs)
}

// realignProbeToConnectFrontier forces block download back to the height ConnectTip needs.
func (s *progressiveRawState) realignProbeToConnectFrontier(bs *BlockStoreCtx, missingHeight int64) {
	if s == nil || bs == nil || bs.Journal == nil || bs.Raw == nil || missingHeight < 0 {
		return
	}
	bs.RefreshContiguousTip()
	j := bs.Journal
	tip, err := j.TipHeight()
	if err != nil || tip < 0 || missingHeight > tip {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.nextProbe
	// PreferConnectFrontierMissing already refuses to collapse the getdata window during
	// deep download-first IBD. realign must use the same guard: yanking nextProbe from
	// ~232k back to chainActive+1 fills every lane with already-stored heights, leaves
	// blocks_stored_ibd at 0, and collapses throughput to ~100 blk/min.
	if prev >= 0 && prev-missingHeight > int64(blockDownloadWindow) {
		if ShouldDeferConnectForBodyDownload(bs) || BodiesBehindHeaders(bs) {
			return
		}
	}
	s.idleFull = false
	s.nextProbe = missingHeight
	if s.nextProbe != prev {
		applog.Line("block", fmt.Sprintf("connect body gap: sync cursor %d → %d (chainActive needs height %d)", prev, s.nextProbe, missingHeight))
		s.persistCheckpointLocked()
	}
}

// realignProbeToLowestMissing resets nextProbe to the first missing height (e.g. after IBD stall).
func (s *progressiveRawState) realignProbeToLowestMissing(bs *BlockStoreCtx) {
	if s == nil || bs == nil || bs.Journal == nil || bs.Raw == nil {
		return
	}
	j := bs.Journal
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.nextProbe
	s.recomputeSyncCursorLocked(j, bs.Raw, bs.ContiguousRawHeight(), tip, bs.chainNet(), ConnectFrontierHeight(bs), bs)
	if !s.idleFull && s.nextProbe != prev {
		applog.Line("block", fmt.Sprintf("IBD stall recovery: sync cursor %d → %d (lowest missing height)", prev, s.nextProbe))
		s.persistCheckpointLocked()
	}
}

// SyncCheckpointToContiguous updates rawblocks_sync.json to the forward download frontier.
func (s *progressiveRawState) SyncCheckpointToContiguous(cont int64) {
	if s == nil || s.chainDir == "" || cont < -1 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var want int64
	if cont < 0 {
		want = 0 // genesis may still be missing (ContiguousRawHeight -1)
	} else {
		want = cont + 1
	}
	if s.nextProbe != want {
		s.nextProbe = want
	}
	s.contiguousCheckpoint = cont
	s.persistCheckpointLocked()
}

func (s *progressiveRawState) OnTipChanged(tip int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onTipChangedLocked(tip)
}

func (s *progressiveRawState) onTipChangedLocked(tip int64) {
	if tip != s.lastTip {
		if tip > s.lastTip {
			s.idleFull = false
		}
		s.lastTip = tip
	}
}

func (s *progressiveRawState) useShortReadDeadline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.idleFull
}

// bodiesDownloadActive is true when forward block IBD should run (guards against stale idleFull).
func (s *progressiveRawState) bodiesDownloadActive(bs *BlockStoreCtx) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	idle := s.idleFull
	s.mu.Unlock()
	if !idle {
		return true
	}
	return bs != nil && BodiesBehindHeaders(bs)
}

func (s *progressiveRawState) laneHasActiveBatch(lane int) bool {
	if s == nil || lane < 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.activeBatch[lane]
	return slot != nil && slot.cancel != nil
}

func (s *progressiveRawState) hasDownloadInFlight() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.inFlight) > 0 {
		return true
	}
	for _, slot := range s.activeBatch {
		if slot != nil && slot.cancel != nil {
			return true
		}
	}
	return false
}

// startBatch begins a getdata window for one lane. If that lane is already downloading,
// it returns ok=false and does not cancel the in-flight batch (relays used to abort assist getdata).
func (s *progressiveRawState) startBatch(lane int, parent context.Context, d time.Duration) (ctx context.Context, end func(), ok bool) {
	s.mu.Lock()
	if s.activeBatch == nil {
		s.activeBatch = make(map[int]*batchSlot)
	}
	if old := s.activeBatch[lane]; old != nil && old.cancel != nil {
		s.mu.Unlock()
		return nil, nil, false
	}
	s.batchGen++
	gen := s.batchGen
	s.mu.Unlock()

	var batchCtx context.Context
	var cancel context.CancelFunc
	if d <= 0 {
		batchCtx, cancel = context.WithCancel(parent)
	} else {
		batchCtx, cancel = context.WithTimeout(parent, d)
	}
	s.mu.Lock()
	if old := s.activeBatch[lane]; old != nil && old.cancel != nil {
		s.mu.Unlock()
		cancel()
		return nil, nil, false
	}
	s.activeBatch[lane] = &batchSlot{gen: gen, cancel: cancel}
	s.mu.Unlock()

	return batchCtx, func() {
		cancel()
		s.mu.Lock()
		if slot := s.activeBatch[lane]; slot != nil && slot.gen == gen {
			delete(s.activeBatch, lane)
		}
		s.mu.Unlock()
	}, true
}

// watchFrontierStall cancels this lane's getdata when Core BLOCK_STALLING_TIMEOUT fires
// while ReadMessage is blocked (live: ahead blocks kept resetting the 30s deadline and
// stall was only checked between batches).
func (s *progressiveRawState) watchFrontierStall(ctx context.Context, bs *BlockStoreCtx, scorer *BlockPeerScorer, book *AddrBook, lane int) func() {
	if s == nil || ctx == nil {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	go func() {
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-t.C:
				if scorer == nil {
					continue
				}
				if peer, stalled := s.maybePenalizeStallingPeer(bs, scorer, book); stalled {
					_ = peer
					_ = lane
					return
				}
			}
		}
	}()
	return stop
}

func (s *progressiveRawState) cancelAllActiveBatches() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for lane, slot := range s.activeBatch {
		if slot != nil && slot.cancel != nil {
			slot.cancel()
		}
		delete(s.activeBatch, lane)
	}
}

func (s *progressiveRawState) recomputeSyncCursorLocked(j *store.HeaderJournal, rs *store.RawBlockStore, contiguous, headerTip int64, net chain.Network, connectNext int64, bs *BlockStoreCtx) {
	tip := capBodyDownloadTip(bs, headerTip)
	if tip < 0 {
		s.idleFull = true
		return
	}
	low, err := store.LowestMissingAfterContiguous(j, rs, contiguous, tip, net)
	if low < 0 && contiguous < tip {
		if contiguous < 0 {
			low = 0
		} else {
			low = contiguous + 1
		}
	}
	low = PreferConnectFrontierMissing(j, rs, low, connectNext, net)
	if err != nil {
		s.idleFull = false
		return
	}
	for h := range s.inFlight {
		if low < 0 || h < low {
			low = h
		}
	}
	if low < 0 {
		s.idleFull = true
		if contiguous >= 0 {
			s.nextProbe = contiguous + 1
		} else {
			s.nextProbe = 0
		}
	} else {
		s.idleFull = false
		s.nextProbe = low
	}
}

// ResumeAfterSnapshotReplay clears replay-idle state and realigns download toward the header tip.
func (s *progressiveRawState) ResumeAfterSnapshotReplay(bs *BlockStoreCtx) {
	if s == nil || bs == nil || bs.Journal == nil || bs.Raw == nil {
		return
	}
	j := bs.Journal
	headerTip, err := j.TipHeight()
	if err != nil || headerTip < 0 {
		return
	}
	cont := bs.ContiguousRawHeight()
	s.mu.Lock()
	prevProbe := s.nextProbe
	wasIdle := s.idleFull
	s.recomputeSyncCursorLocked(j, bs.Raw, cont, headerTip, bs.chainNet(), ConnectFrontierHeight(bs), bs)
	if cont < headerTip {
		s.idleFull = false
	}
	s.persistCheckpointLocked()
	probe := s.nextProbe
	idle := s.idleFull
	s.mu.Unlock()
	if wasIdle && !idle && cont < headerTip {
		applog.Line("block", fmt.Sprintf("snapshot replay done: resuming forward body IBD toward header height %d (bodies through %d, probe %d → %d)",
			headerTip, cont, prevProbe, probe))
	}
}

// pickParallelChunkClaim assigns a disjoint ~getdata-sized height chunk to this lane.
// Gap-fill: when contiguous+1 is free and this lane may own it, claim from the hole first.
// Ahead-only chunk stripes (chunkLane>=1) never include lowMissing, so soft-stall/idle
// assist reclaim must plan the hole range explicitly — otherwise the window fills with
// ahead remnants while the hole stays orphaned.
func (s *progressiveRawState) pickParallelChunkClaim(
	bs *BlockStoreCtx,
	j *store.HeaderJournal,
	rs *store.RawBlockStore,
	workerID, workers, batchCap int,
	lowMissing, probeStart, stripeTip, downloadTip int64,
	laneInflight int,
	inFlightSnap map[int64][32]byte,
) rawBatchClaim {
	var best rawBatchClaim
	if batchCap < 1 {
		batchCap = progressiveBatchSize
	}
	maxSlots := workers * 8
	if maxSlots < 8 {
		maxSlots = 8
	}
	minPrefer := batchCap / 4
	if minPrefer < 32 {
		minPrefer = 32
	}
	chunkLane := chunkLaneForWorker(workerID, workers)
	mayHole := s.mayClaimContiguousHole(workerID, workers, lowMissing, inFlightSnap)
	holeFree := true
	if _, busy := inFlightSnap[lowMissing]; busy {
		holeFree = false
	}
	if mayHole && holeFree {
		holeBatch := s.holeFillBatchSize(lowMissing, batchCap)
		holeHi := lowMissing + int64(holeBatch) - 1
		if holeHi > stripeTip {
			holeHi = stripeTip
		}
		if holeHi > downloadTip {
			holeHi = downloadTip
		}
		probe := lowMissing
		if probeStart > probe {
			probe = probeStart
		}
		best = s.planClaimRange(bs, j, rs, probe, lowMissing, holeHi, downloadTip, lowMissing, workerID, workers, laneInflight, inFlightSnap)
		if len(best.heights) > 0 {
			return best
		}
		// Hole heights already on disk / skipped — do not fall through to ahead while
		// contiguous tip still reports this lowMissing (planner will advance next tick).
		return best
	}
	for slot := 0; slot < maxSlots; slot++ {
		stripeLo, stripeHi, ok := syncBatchChunkBounds(lowMissing, stripeTip, chunkLane, workers, batchCap, slot)
		if !ok {
			break
		}
		coversHole := stripeLo <= lowMissing && lowMissing <= stripeHi
		if coversHole && !mayHole {
			continue
		}
		if mayHole && holeFree && !coversHole {
			continue
		}
		if chunkLane != 0 && coversHole && !mayHole {
			continue
		}
		probe := stripeLo
		if probeStart > probe {
			probe = probeStart
		}
		cand := s.planClaimRange(bs, j, rs, probe, stripeLo, stripeHi, downloadTip, lowMissing, workerID, workers, laneInflight, inFlightSnap)
		if len(cand.heights) == 0 {
			continue
		}
		if coversHole || (cand.lo <= lowMissing && lowMissing <= cand.hi) {
			return cand
		}
		if mayHole && holeFree {
			continue
		}
		if len(cand.heights) >= minPrefer {
			return cand
		}
		if len(best.heights) == 0 || len(cand.heights) > len(best.heights) {
			best = cand
		}
	}
	return best
}

// claimBatch reserves up to progressiveBatchSize consecutive missing heights for one sync lane.
func (s *progressiveRawState) claimBatch(bs *BlockStoreCtx, workerID int) (rawBatchClaim, bool) {
	var empty rawBatchClaim
	if bs == nil || bs.Raw == nil || bs.Journal == nil {
		return empty, false
	}
	s.ensureBodyDownloadArmed(bs)
	s.mu.Lock()
	if s.inFlight == nil {
		s.inFlight = make(map[int64][32]byte)
	}
	if s.inFlightLane == nil {
		s.inFlightLane = make(map[int64]int)
	}
	idle := s.idleFull
	diskPaused := s.diskPressurePaused
	if workerID < 0 {
		workerID = 0
	}
	workers := s.syncWorkers
	if workers < 1 {
		workers = 1
	}
	if workers > maxBlockAssistWorkers+1 {
		workers = maxBlockAssistWorkers + 1
	}
	nextProbe := s.nextProbe
	inFlightSnap := make(map[int64][32]byte, len(s.inFlight))
	for h, hash := range s.inFlight {
		inFlightSnap[h] = hash
	}
	laneInflight := s.inFlightCountForLaneLocked(workerID)
	s.mu.Unlock()

	j := bs.Journal
	rs := bs.Raw
	if idle || diskPaused {
		return empty, false
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return empty, false
	}
	s.mu.Lock()
	s.onTipChangedLocked(tip)
	if s.nextProbe < 0 {
		s.nextProbe = 0
	}
	if nextProbe < 0 {
		nextProbe = 0
	}
	s.mu.Unlock()

	contiguous := bs.ContiguousRawHeight()
	downloadTip := capBodyDownloadTip(bs, tip)
	lowMissing, err := LowestMissingForIBD(j, rs, contiguous, tip, bs)
	if err != nil {
		return empty, false
	}
	// Drop claims already behind the contiguous tip (yanked cursor / hybrid re-claims).
	// Then NEVER redefine lowMissing as min(inFlight): when the hole is free and ahead
	// heights are claimed, min(inFlight) skips contiguous+1 and saturates the window so
	// claimBatch returns empty forever (0 blk/min with peers still connected).
	s.mu.Lock()
	if freed := s.releaseStaleInFlightBelowContiguousLocked(contiguous); freed > 0 {
		inFlightSnap = make(map[int64][32]byte, len(s.inFlight))
		for h, hash := range s.inFlight {
			inFlightSnap[h] = hash
		}
		laneInflight = s.inFlightCountForLaneLocked(workerID)
	}
	s.mu.Unlock()
	if lowMissing >= 1 && nextProbe > lowMissing {
		nextProbe = lowMissing
	}
	probeStart := lowMissing
	if probeStart < 0 {
		probeStart = 0
	}
	if lowMissing < 0 {
		s.mu.Lock()
		s.idleFull = true
		s.nextProbe = tip + 1
		s.persistCheckpointLocked()
		s.mu.Unlock()
		return empty, false
	}
	win := int(ibdBodyFetchWindow(bs, workers))
	if win < progressiveBatchSize {
		win = progressiveBatchSize
	}
	batchCap := s.peerInFlightBudget(bs, workerID)
	if batchCap < 1 {
		batchCap = progressiveBatchSize
	}
	useChunks := workers > 1 && shouldUseParallelBatchChunks(bs, lowMissing)
	// Keep Core-sized headroom for the contiguous hole when not using parallel chunks.
	frontierReserve := progressiveBatchSize
	if len(inFlightSnap) >= win {
		if _, busy := inFlightSnap[lowMissing]; busy {
			return empty, false
		}
		// Window saturated with ahead claims but hole is free: still allow a frontier
		// claim so gap-fill is not starved by ahead getdata.
	}
	if !useChunks && len(inFlightSnap) >= win-frontierReserve && lowMissing >= 0 {
		maxHi := lowMissing + int64(frontierReserve) - 1
		if downloadTip > maxHi {
			downloadTip = maxHi
		}
	}
	stripeTip := forwardIBDStripeTipFor(bs, lowMissing, downloadTip, workers)

	var claim rawBatchClaim
	if useChunks {
		claim = s.pickParallelChunkClaim(bs, j, rs, workerID, workers, batchCap, lowMissing, probeStart, stripeTip, downloadTip, laneInflight, inFlightSnap)
	} else {
		stripeWorkers := workers
		if shouldFillContiguousFrontierFirst(bs, lowMissing) {
			stripeWorkers = 1
		}
		stripeID := workerID
		if stripeWorkers == 1 || workerID >= stripeWorkers {
			stripeWorkers = 1
			stripeID = 0
		}
		stripeLo, stripeHi, ok := syncStripeBounds(lowMissing, stripeTip, stripeID, stripeWorkers)
		if !ok {
			return empty, false
		}
		rangeLo, rangeHi := stripeLo, stripeHi
		probe := probeStart
		if probe < rangeLo {
			probe = rangeLo
		}
		claim = s.planClaimRange(bs, j, rs, probe, rangeLo, rangeHi, downloadTip, lowMissing, workerID, workers, laneInflight, inFlightSnap)
		if len(claim.heights) == 0 && workers > 1 && stripeWorkers > 1 {
			stripeMissing, err := rangeHasMissingBlock(j, rs, stripeLo, stripeHi, bs.chainNet())
			if err != nil {
				return empty, false
			}
			if !stripeMissing {
				restart := stripeLo
				if lowMissing > restart {
					restart = lowMissing
				}
				claim = s.planClaimRange(bs, j, rs, restart, lowMissing, stripeTip, downloadTip, lowMissing, workerID, workers, laneInflight, inFlightSnap)
				if len(claim.heights) > 0 {
					applog.Line("block", fmt.Sprintf("sync lane %d/%d rebalanced to heights %d..%d (stripe %d..%d complete)", workerID, workers, claim.lo, claim.hi, stripeLo, stripeHi))
				}
			}
		}
	}
	if len(claim.heights) == 0 {
		s.mu.Lock()
		s.recomputeSyncCursorLocked(j, rs, bs.ContiguousRawHeight(), tip, bs.chainNet(), ConnectFrontierHeight(bs), bs)
		s.persistCheckpointLocked()
		s.mu.Unlock()
		return empty, false
	}
	rangeLo, rangeHi := claim.lo, claim.hi
	probeStart = claim.lo
	for attempt := 0; attempt < workers; attempt++ {
		if attempt > 0 {
			s.mu.Lock()
			inFlightSnap = make(map[int64][32]byte, len(s.inFlight))
			for h, hash := range s.inFlight {
				inFlightSnap[h] = hash
			}
			laneInflight = s.inFlightCountForLaneLocked(workerID)
			s.mu.Unlock()
			if useChunks {
				claim = s.pickParallelChunkClaim(bs, j, rs, workerID, workers, batchCap, lowMissing, probeStart, stripeTip, downloadTip, laneInflight, inFlightSnap)
			} else {
				claim = s.planClaimRange(bs, j, rs, probeStart, rangeLo, rangeHi, downloadTip, lowMissing, workerID, workers, laneInflight, inFlightSnap)
			}
			if len(claim.heights) == 0 {
				return empty, false
			}
		}
		s.mu.Lock()
		var keptH []int64
		var keptHash [][32]byte
		for i, h := range claim.heights {
			if _, busy := s.inFlight[h]; busy {
				continue
			}
			keptH = append(keptH, h)
			keptHash = append(keptHash, claim.hashes[i])
		}
		claim.heights = keptH
		claim.hashes = keptHash
		if len(claim.heights) == 0 {
			s.mu.Unlock()
			continue
		}
		for i, h := range claim.heights {
			s.inFlight[h] = claim.hashes[i]
			s.inFlightLane[h] = workerID
		}
		s.noteBatchDownloadStartLocked(workerID)
		claim.lo = claim.heights[0]
		claim.hi = claim.heights[len(claim.heights)-1]
		s.mu.Unlock()
		return claim, true
	}
	return empty, false
}

func (s *progressiveRawState) laneHoldsFrontier(bs *BlockStoreCtx, lane int) bool {
	if s == nil || bs == nil || lane < 0 {
		return false
	}
	cont := bs.ContiguousRawHeight()
	frontier := cont + 1
	if cont < 0 {
		frontier = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.inFlight[frontier]; !ok {
		return false
	}
	if s.inFlightLane == nil {
		return false
	}
	return s.inFlightLane[frontier] == lane
}

func (s *progressiveRawState) inFlightCountForLaneLocked(lane int) int {
	n := 0
	for _, l := range s.inFlightLane {
		if l == lane {
			n++
		}
	}
	return n
}

func (s *progressiveRawState) releaseInFlightHeight(h int64) {
	if s == nil || h < 0 {
		return
	}
	s.mu.Lock()
	delete(s.inFlight, h)
	delete(s.inFlightLane, h)
	s.mu.Unlock()
}

func mergeRawBatchClaims(base rawBatchClaim, extra []rawBatchClaim) rawBatchClaim {
	for _, e := range extra {
		if len(e.heights) == 0 {
			continue
		}
		base.heights = append(base.heights, e.heights...)
		base.hashes = append(base.hashes, e.hashes...)
		if e.hi > base.hi {
			base.hi = e.hi
		}
	}
	return base
}

// planClaimRange selects missing heights without holding progressiveRawState.mu or mutating in-flight maps.
func (s *progressiveRawState) planClaimRange(bs *BlockStoreCtx, j *store.HeaderJournal, rs *store.RawBlockStore, probeStart, rangeLo, rangeHi, tip, lowMissing int64, workerID, workers, laneInflight int, inFlight map[int64][32]byte) rawBatchClaim {
	var claim rawBatchClaim
	if probeStart < rangeLo {
		probeStart = rangeLo
	}
	if workers < 1 {
		workers = 1
	}
	batchCap := s.peerInFlightBudget(bs, workerID)
	if batchCap < 1 {
		batchCap = EffectiveProgressiveBatchSizeForIBD(bs, workers)
	}
	if laneInflight >= batchCap {
		return claim
	}
	maxNew := batchCap - laneInflight
	if j == nil {
		return claim
	}
	cont := int64(-2)
	if bs != nil {
		cont = bs.ContiguousRawHeight()
	}
	contSkip := cont
	// If cont==0 but genesis raw is missing, treat cont as unknown so we
	// still allow claiming height 0.
	if contSkip == 0 && rs != nil && j != nil {
		if h80, err := j.ReadHeaderAt(0); err == nil && len(h80) >= 80 {
			genHash := pow.BlockHashLE(h80)
			if !rs.Has(genHash) {
				contSkip = -1
			}
		}
	}
	if contSkip >= 0 && probeStart <= contSkip {
		probeStart = contSkip + 1
	}
	skipDisk := shouldSkipDiskBodyProbe(bs)
	// During download-first IBD skip HasStoredBody Stat. Still skip RAM/locator-present
	// bodies so lanes do not burn the window re-fetching orphans already on disk.
	walkCap := maxNew * 2
	if skipDisk {
		walkCap = int(rangeHi - probeStart + 1)
		if walkCap < maxNew {
			walkCap = maxNew
		}
		if walkCap > 8192 {
			walkCap = 8192
		}
	} else {
		if walkCap < 64 {
			walkCap = 64
		}
		if walkCap > 512 {
			walkCap = 512
		}
	}
	scanned := 0
	for probe := probeStart; probe <= rangeHi && probe <= tip && len(claim.heights) < maxNew; probe++ {
		if contSkip >= 0 && probe <= contSkip {
			continue
		}
		if _, busy := inFlight[probe]; busy {
			continue
		}
		scanned++
		if scanned > walkCap {
			break
		}
		h80, err := j.ReadHeaderAt(probe)
		if err != nil {
			return claim
		}
		hash := pow.BlockHashLE(h80)
		if rs != nil {
			net := chain.MainnetDogecoin
			if bs != nil {
				net = bs.chainNet()
			}
			minB := store.MinRawBlockBytes(net, probe)
			if skipDisk {
				if rs.LikelyHasBody(hash, minB) {
					continue
				}
			} else if rs.HasStoredBody(hash, minB) {
				continue
			}
		}
		claim.heights = append(claim.heights, probe)
		claim.hashes = append(claim.hashes, hash)
	}
	if len(claim.heights) > 0 {
		claim.lo = claim.heights[0]
		claim.hi = claim.heights[len(claim.heights)-1]
	}
	return claim
}

// scanClaimRange claims a contiguous batch of missing heights within [rangeLo, rangeHi].
func (s *progressiveRawState) scanClaimRange(bs *BlockStoreCtx, j *store.HeaderJournal, rs *store.RawBlockStore, probeStart, rangeLo, rangeHi, tip, lowMissing int64, workerID, workers int) rawBatchClaim {
	inFlightSnap := make(map[int64][32]byte, len(s.inFlight))
	for h, hash := range s.inFlight {
		inFlightSnap[h] = hash
	}
	claim := s.planClaimRange(bs, j, rs, probeStart, rangeLo, rangeHi, tip, lowMissing, workerID, workers, s.inFlightCountForLaneLocked(workerID), inFlightSnap)
	for i, h := range claim.heights {
		s.inFlight[h] = claim.hashes[i]
		if s.inFlightLane == nil {
			s.inFlightLane = make(map[int64]int)
		}
		s.inFlightLane[h] = workerID
	}
	return claim
}

// finishBatch releases in-flight heights and updates sync cursor after a getdata batch completes.
func (s *progressiveRawState) finishBatch(bs *BlockStoreCtx, claim rawBatchClaim, stored int, fetchErr error) (fetched bool) {
	if bs == nil || bs.Journal == nil || bs.Raw == nil || len(claim.heights) == 0 {
		return stored > 0
	}
	j := bs.Journal
	rs := bs.Raw
	lane := -1
	s.mu.Lock()
	if len(claim.heights) > 0 && s.inFlightLane != nil {
		if l, ok := s.inFlightLane[claim.heights[0]]; ok {
			lane = l
		}
	}
	for _, h := range claim.heights {
		delete(s.inFlight, h)
		delete(s.inFlightLane, h)
	}
	if lane >= 0 {
		s.clearLaneDownloadIfIdleLocked(lane)
	}
	s.mu.Unlock()

	tip, err := j.TipHeight()
	if err != nil {
		return stored > 0
	}
	cont := bs.ContiguousRawHeight()
	connect := stored > 0
	if bs.utxoAheadOfStoredBodies() {
		if after := rampReplayContiguousFromDiskBounded(bs, 4); after > cont {
			cont = after
			connect = true
		}
	}
	s.mu.Lock()
	s.recomputeSyncCursorLocked(j, rs, cont, tip, bs.chainNet(), ConnectFrontierHeight(bs), bs)
	s.persistCheckpointLocked()
	if stored > 0 {
		s.noteBlocksStoredLocked(stored)
		s.stallingSince = time.Time{}
	}
	s.noteContiguousTipLocked(cont)
	s.mu.Unlock()
	if connect {
		bs.FlushDeferredConnect()
		if bs.Utxo != nil && shouldPostBatchInlineConnect(bs) {
			if err := bs.SyncUtxoCache(); err != nil {
				applog.Line("utxo", "post-batch sync: "+err.Error())
			}
		}
		if cont := bs.ContiguousRawHeight(); cont >= 0 {
			if fn := bs.onContiguousAdvance; fn != nil {
				fn(cont)
			}
		}
	}
	if fetchErr != nil && shouldRotatePeerForStubBlock(fetchErr) && bs != nil {
		removed := 0
		lowest := int64(-1)
		for _, h := range claim.heights {
			if ok, err := bs.purgeUnreadableBodyAtHeight(h); err != nil {
				applog.Line("block", "stub purge: "+err.Error())
			} else if ok {
				removed++
				if lowest < 0 || h < lowest {
					lowest = h
				}
			}
		}
		if removed > 0 {
			applog.Line("block", fmt.Sprintf("stub purge: removed %d undersized raw block(s) from batch after failed getdata", removed))
			if bs.utxoAheadOfStoredBodies() {
				if lowest >= 0 {
					bs.shrinkContiguousTipAfterBodyRemoved(lowest)
				}
			} else {
				bs.RevalidateContiguousTip()
			}
		}
	}
	if fetchErr != nil && stored == 0 {
		return false
	}
	return stored > 0
}

// tryFetchOneMissing claims a batch, downloads it, and releases the claim.
// Returns the number of blocks stored (0 if none claimed).
func (s *progressiveRawState) tryFetchOneMissing(ctx context.Context, w *MsgWriter, p chain.Params, bs *BlockStoreCtx, workerID int, scorer *BlockPeerScorer) (stored int, err error) {
	return s.tryFetchMissingBatches(ctx, w, p, bs, workerID, 1, scorer, nil)
}

// tryFetchMissingBatches runs up to maxBatches progressive getdata rounds (Core interleaves during header wait).
func (s *progressiveRawState) tryFetchMissingBatches(ctx context.Context, w *MsgWriter, p chain.Params, bs *BlockStoreCtx, workerID, maxBatches int, scorer *BlockPeerScorer, book *AddrBook) (stored int, err error) {
	if maxBatches < 1 {
		maxBatches = 1
	}
	if workerID < 0 {
		return 0, nil
	}
	s.ensureBodyDownloadArmed(bs)
	var total int
	var lastErr error
	s.releaseOrphanInFlight()
	for batch := 0; batch < maxBatches; batch++ {
		if w != nil && w.PeerAddr != "" {
			s.noteLanePeer(workerID, w.PeerAddr)
		}
		if scorer != nil {
			if stallPeer, stalled := s.maybePenalizeStallingPeer(bs, scorer, book); stalled {
				// Soft-stall frees the hole for another lane (stallPeer==""). Hard-stall
				// disconnects only the peer that held the frontier — other lanes must keep
				// downloading ahead / reclaim the hole instead of suiciding.
				if stallPeer != "" {
					self := ""
					if w != nil {
						self = w.PeerAddr
					}
					if self != "" && self == stallPeer {
						return total, blockStallError(stallPeer)
					}
					// Observer lane: hole was released or another peer will disconnect; continue.
					continue
				}
				continue
			}
			if timeoutPeer, timedOut := s.maybePenalizeDownloadTimeout(bs, scorer, book, workerID); timedOut {
				return total, blockDownloadTimeoutError(timeoutPeer)
			}
		}
		if s.laneHasActiveBatch(workerID) {
			break
		}
		lanes := s.syncWorkerCount()
		batchTimeout := time.Duration(0)
		if !shouldPipelineGetData(bs) {
			s.mu.Lock()
			batchTimeout = s.effectiveLaneDownloadTimeoutLocked(bs, lanes, workerID)
			s.mu.Unlock()
		}
		batchCtx, endBatch, started := s.startBatch(workerID, ctx, batchTimeout)
		if !started {
			break
		}
		stopWatch := func() {}
		if scorer != nil {
			stopWatch = s.watchFrontierStall(batchCtx, bs, scorer, book, workerID)
		}
		claim, ok := s.claimBatch(bs, workerID)
		if !ok {
			stopWatch()
			endBatch()
			break
		}
		if s.syncWorkers > 1 {
			applog.Line("block", fmt.Sprintf("progressive getdata heights %d..%d (%d block(s), lane %d/%d)", claim.lo, claim.hi, len(claim.hashes), workerID, s.syncWorkers))
		} else {
			applog.Line("block", fmt.Sprintf("progressive getdata heights %d..%d (%d block(s))", claim.lo, claim.hi, len(claim.hashes)))
		}
		NoteBlockGetdata(claim.lo, claim.hi, workerID)
		var extra []rawBatchClaim
		var hooks *getdataBatchHooks
		if shouldPipelineGetData(bs) {
			budget := s.peerInFlightBudget(bs, workerID)
			peerAddr := ""
			if w != nil {
				peerAddr = w.PeerAddr
			}
			hooks = &getdataBatchHooks{
				RefillBelow: getdataRefillThreshold(budget),
				ProgressDownloadTimeout: func() time.Duration {
					s.mu.Lock()
					defer s.mu.Unlock()
					return s.effectiveLaneDownloadTimeoutLocked(bs, lanes, workerID)
				},
				OnStored: func(h int64) {
					s.releaseInFlightHeight(h)
					s.mu.Lock()
					s.noteLaneBlockReceivedLocked(workerID)
					s.mu.Unlock()
					if scorer != nil && peerAddr != "" {
						scorer.NoteBlocksDelivered(peerAddr, 1)
					}
				},
				Refill: func(pending int) ([][32]byte, []int64) {
					b := s.peerInFlightBudget(bs, workerID)
					if !shouldRefillGetDataAt(pending, b) {
						return nil, nil
					}
					more, ok := s.claimBatch(bs, workerID)
					if !ok || len(more.heights) == 0 {
						return nil, nil
					}
					extra = append(extra, more)
					return more.hashes, more.heights
				},
			}
		}
		n, ferr := fetchAndStoreRawBlocksBatch(batchCtx, w, p, claim.hashes, claim.heights, bs, lanes, hooks)
		stopWatch()
		endBatch()
		claim = mergeRawBatchClaims(claim, extra)
		if n > 0 {
			s.mu.Lock()
			if hooks == nil {
				s.noteLaneBlocksDeliveredLocked(workerID, n)
			}
			s.noteLaneDownloadProgressLocked(workerID)
			s.mu.Unlock()
			if hooks == nil && scorer != nil && w != nil && w.PeerAddr != "" {
				scorer.NoteBlocksDelivered(w.PeerAddr, n)
			}
		}
		if ferr != nil && (errors.Is(ferr, context.Canceled) || errors.Is(ferr, context.DeadlineExceeded)) {
			s.mu.Lock()
			stallPeer := s.lastStallPeer
			stallAt := s.lastStallAt
			s.mu.Unlock()
			if !stallAt.IsZero() && time.Since(stallAt) < 3*time.Second {
				s.finishBatch(bs, claim, n, ferr)
				return total + n, blockStallError(stallPeer)
			}
		}
		if ferr != nil && n == 0 {
			peer := ""
			if w != nil {
				peer = w.PeerAddr
			}
			applog.Line("block", fmt.Sprintf("progressive getdata heights %d..%d failed on %s: %v", claim.lo, claim.hi, peer, ferr))
			if shouldRotatePeerForStubBlock(ferr) && peer != "" {
				penalizeStubBlockPeer(scorer, book, peer)
			} else if peer != "" && scorer != nil && shouldRotatePeerForForwardIBDFetch(ferr, claim.lo) {
				penalizeBlockPeer(scorer, book, peer, true)
			}
		}
		if s.finishBatch(bs, claim, n, ferr) {
			total += n
			lastErr = ferr
		}
		if n <= 0 {
			if ferr != nil {
				lastErr = ferr
			}
			break
		}
	}
	return total, lastErr
}

// maybeRepairTxIndex runs an idempotent tx index rebuild when the index lags raw blocks or sampled parent lookups are missing.
func maybeRepairTxIndex(chainDir string, bs *BlockStoreCtx, minRawBlocks int) {
	if chainDir == "" || minRawBlocks <= 0 {
		return
	}
	if bs != nil && bs.Journal != nil && bs.Raw != nil && bs.TxIndex != nil {
		through := int64(-1)
		if bs.Utxo != nil {
			through = bs.Utxo.TipHeight()
		}
		if through < 0 {
			through = bs.ContiguousRawHeight()
		}
		if through >= 0 {
			rep, ran, err := store.RepairTxIndexIfSparse(chainDir, bs.Journal, bs.Raw, bs.TxIndex, bs.chainNet(), through, 128, minRawBlocks)
			if err != nil {
				applog.Line("block", "tx index sparse repair: "+err.Error())
			} else if ran {
				applog.Line("block", fmt.Sprintf("tx index sparse repair: re-indexed %d block(s), %d tx file(s) (sampled through height %d)", rep.BlocksIndexed, rep.TxFiles, through))
				return
			}
		}
	}
	rep, ran, err := store.RepairTxIndexIfLag(chainDir, minRawBlocks)
	if err != nil {
		applog.Line("block", "tx index repair: "+err.Error())
		return
	}
	if ran {
		applog.Line("block", fmt.Sprintf("tx index repair: re-indexed %d block(s), %d tx file(s)", rep.BlocksIndexed, rep.TxFiles))
	}
}

// maybeUpgradeLegacyTxIndex rewrites 36-byte tx index files to v2 (embedded tx raw) in small batches.
func maybeUpgradeLegacyTxIndex(chainDir string, batch int) {
	if chainDir == "" || batch <= 0 {
		return
	}
	upgraded, remaining, err := store.UpgradeLegacyTxIndexBatch(chainDir, batch)
	if err != nil {
		applog.Line("block", "tx index v2 upgrade: "+err.Error())
		return
	}
	if upgraded > 0 {
		applog.Line("block", fmt.Sprintf("tx index v2 upgrade: %d file(s) upgraded (%d legacy remaining)", upgraded, remaining))
	}
}

const txIndexRepairMinRawBlocks = 32
const txIndexLegacyUpgradeBatch = 256

// snapshot returns a copy of sync coordinator state for RPC / dashboard.
func (s *progressiveRawState) snapshot() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]interface{}{
		"next_probe_height": s.nextProbe,
		"idle_full":         s.idleFull,
		"disk_pressure_paused": s.diskPressurePaused,
		"in_flight_batches": len(s.inFlight),
		"blocks_in_flight":  len(s.inFlight),
		"last_header_tip":   s.lastTip,
		"sync_workers":      s.syncWorkers,
		"blocks_stored_ibd": s.blocksStoredIBD,
	}
	if !s.ibdStarted.IsZero() {
		elapsed := time.Since(s.ibdStarted)
		out["ibd_elapsed_sec"] = int64(elapsed.Seconds())
		lifetimeBPM := float64(0)
		if elapsed >= time.Minute && s.blocksStoredIBD > 0 {
			lifetimeBPM = float64(s.blocksStoredIBD) / elapsed.Minutes()
			out["blocks_per_minute_lifetime"] = lifetimeBPM
		}
		// Prefer recent-window rate so stalls early in IBD do not dilute the live dashboard number.
		if recent := s.recentBlocksPerMinuteLocked(); recent > 0 {
			out["blocks_per_minute"] = recent
		} else if lifetimeBPM > 0 {
			out["blocks_per_minute"] = lifetimeBPM
		}
		if contigRate := s.recentContiguousBlocksPerMinuteLocked(); contigRate > 0 {
			out["contiguous_blocks_per_minute"] = contigRate
		}
	}
	if !s.lastStoredAt.IsZero() {
		out["last_block_stored_at"] = s.lastStoredAt.Unix()
	}
	if hdrRate := RecentHeadersPerMinute(); hdrRate > 0 {
		out["headers_per_minute"] = hdrRate
	}
	lanes := s.syncWorkers
	if lanes < 1 {
		lanes = 1
	}
	if lanes > maxBlockAssistWorkers+1 {
		lanes = maxBlockAssistWorkers + 1
	}
	dl := BlockDownloadTimeout(lanes-1, 60)
	if len(s.inFlight) > 0 && dl > bodyIBDBlockDownloadTimeout {
		dl = bodyIBDBlockDownloadTimeout
	}
	out["block_download_timeout_sec"] = int64(dl.Seconds())
	out["block_stalling_timeout_sec"] = int64(blockStallingTimeout.Seconds())
	out["block_stalling_timeout_body_ibd_sec"] = int64(blockStallingTimeoutBodyIBD.Seconds())
	out["block_stalling_timeout_body_ibd_early_sec"] = int64(blockStallingTimeoutBodyIBDEarly.Seconds())
	if !s.stallingSince.IsZero() {
		out["frontier_stalling_since"] = s.stallingSince.Unix()
	}
	if s.lastStallPeer != "" && !s.lastStallAt.IsZero() {
		out["last_block_stall_peer"] = s.lastStallPeer
		out["last_block_stall_at"] = s.lastStallAt.Unix()
	}
	if s.lastDownloadTimeoutPeer != "" && !s.lastDownloadTimeoutAt.IsZero() {
		out["last_block_download_timeout_peer"] = s.lastDownloadTimeoutPeer
		out["last_block_download_timeout_at"] = s.lastDownloadTimeoutAt.Unix()
	}
	out["max_blocks_in_transit_per_peer"] = ibdPeerInFlightMax
	if len(s.inFlightLane) > 0 {
		laneInflight := make(map[string]int)
		for _, lane := range s.inFlightLane {
			key := fmt.Sprintf("lane_%d", lane)
			if s.laneAddr != nil {
				if addr, ok := s.laneAddr[lane]; ok && addr != "" {
					key = addr
				}
			}
			laneInflight[key]++
		}
		out["lane_in_flight"] = laneInflight
	}
	if budgets := s.laneBudgetSnapshotLocked(nil); len(budgets) > 0 {
		out["lane_budget"] = budgets
	}
	return out
}
