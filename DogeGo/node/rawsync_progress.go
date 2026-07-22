// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"hash/fnv"
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
	mu          sync.Mutex
	chainDir    string
	lastTip     int64
	nextProbe   int64 // scan start for gaps (ascending)
	idleFull    bool
	inFlight     map[int64][32]byte
	inFlightLane map[int64]int // height → sync lane (0 = primary)
	syncWorkers  int           // parallel download lanes (primary + block-assist); 1 = no striping

	ibdStarted       time.Time
	blocksStoredIBD  int64
	lastStoredAt     time.Time
	rateSamples      []ibdRateSample // recent window for stable blk/min (not lifetime mean)

	laneAddr      map[int]string // sync lane → peer host:port (block stall detection)
	stallingSince time.Time      // Core nStallingSince when frontier height is in-flight
	lastStallPeer string         // last peer penalized for block stalling (RPC snapshot)
	lastStallAt   time.Time

	laneDownloadSince       map[int]time.Time // per sync lane: first getdata in current batch
	lastDownloadTimeoutPeer string
	lastDownloadTimeoutAt   time.Time

	contiguousCheckpoint int64 // persisted monotonic raw coverage; -2 = not loaded

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
	ibdRateMinWindow = 45 * time.Second
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

// laneForAddr picks a sync lane for a non-primary P2P link (lane 0 is reserved for the primary peer).
func (s *progressiveRawState) laneForAddr(addr string) int {
	n := s.syncWorkerCount()
	if n <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(addr))
	slot := 1 + int(h.Sum32()%uint32(n-1))
	if slot >= n {
		slot = n - 1
	}
	return slot
}

// SetSyncParallelism configures height stripes for parallel getdata (include primary link in the count).
func (s *progressiveRawState) SetSyncParallelism(workers int) {
	if s == nil || workers < 1 {
		return
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
	if freed > 0 {
		s.idleFull = false
		applog.Line("block", fmt.Sprintf("released %d in-flight block height(s) on sync lane %d (peer disconnected)", freed, lane))
	}
	delete(s.laneDownloadSince, lane)
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

func (s *progressiveRawState) startBatch(lane int, parent context.Context, d time.Duration) (context.Context, func()) {
	s.mu.Lock()
	s.batchGen++
	gen := s.batchGen
	if s.activeBatch == nil {
		s.activeBatch = make(map[int]*batchSlot)
	}
	if old := s.activeBatch[lane]; old != nil && old.cancel != nil {
		old.cancel()
	}
	s.mu.Unlock()

	batchCtx, cancel := context.WithTimeout(parent, d)
	s.mu.Lock()
	s.activeBatch[lane] = &batchSlot{gen: gen, cancel: cancel}
	s.mu.Unlock()

	return batchCtx, func() {
		cancel()
		s.mu.Lock()
		if slot := s.activeBatch[lane]; slot != nil && slot.gen == gen {
			delete(s.activeBatch, lane)
		}
		s.mu.Unlock()
	}
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
	if low < 0 {
		searchStart := store.LowestMissingSearchStart(j, rs, contiguous, net)
		low, err = store.LowestMissingBlockHeightFrom(j, rs, searchStart, tip, net)
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

// claimBatch reserves up to progressiveBatchSize consecutive missing heights for one sync lane.
func (s *progressiveRawState) claimBatch(bs *BlockStoreCtx, workerID int) (rawBatchClaim, bool) {
	var empty rawBatchClaim
	if bs == nil || bs.Raw == nil || bs.Journal == nil {
		return empty, false
	}
	s.mu.Lock()
	if s.inFlight == nil {
		s.inFlight = make(map[int64][32]byte)
	}
	if s.inFlightLane == nil {
		s.inFlightLane = make(map[int64]int)
	}
	idle := s.idleFull
	workers := s.syncWorkers
	if workers < 1 {
		workers = 1
	}
	if workerID < 0 || workerID >= workers {
		workerID = 0
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
	if idle {
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
	j.ReconcileCountCacheFromDisk()
	downloadTip := capBodyDownloadTip(bs, tip)
	lowMissing, err := LowestMissingForIBD(j, rs, contiguous, tip, bs)
	if err != nil {
		return empty, false
	}
	for h := range inFlightSnap {
		if lowMissing < 0 || h < lowMissing {
			lowMissing = h
		}
	}
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
	stripeTip := forwardIBDStripeTip(bs, lowMissing, downloadTip)
	stripeWorkers := workers
	if shouldFillContiguousFrontierFirst(bs, lowMissing) {
		stripeWorkers = 1
	}
	stripeID := workerID
	if stripeWorkers == 1 {
		stripeID = 0
	}
	stripeLo, stripeHi, ok := syncStripeBounds(lowMissing, stripeTip, stripeID, stripeWorkers)
	if !ok {
		return empty, false
	}
	rangeLo, rangeHi := stripeLo, stripeHi
	if probeStart < rangeLo {
		probeStart = rangeLo
	}
	claim := s.planClaimRange(bs, j, rs, probeStart, rangeLo, rangeHi, downloadTip, lowMissing, workerID, workers, laneInflight, inFlightSnap)
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
	if len(claim.heights) == 0 {
		s.mu.Lock()
		s.recomputeSyncCursorLocked(j, rs, bs.ContiguousRawHeight(), tip, bs.chainNet(), ConnectFrontierHeight(bs), bs)
		s.persistCheckpointLocked()
		s.mu.Unlock()
		return empty, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, h := range claim.heights {
		if _, busy := s.inFlight[h]; busy {
			claim.heights = claim.heights[:i]
			claim.hashes = claim.hashes[:i]
			break
		}
	}
	if len(claim.heights) == 0 {
		return empty, false
	}
	for i, h := range claim.heights {
		s.inFlight[h] = claim.hashes[i]
		s.inFlightLane[h] = workerID
	}
	s.noteBatchDownloadStartLocked(workerID)
	claim.lo = claim.heights[0]
	claim.hi = claim.heights[len(claim.heights)-1]
	return claim, true
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

// planClaimRange selects missing heights without holding progressiveRawState.mu or mutating in-flight maps.
func (s *progressiveRawState) planClaimRange(bs *BlockStoreCtx, j *store.HeaderJournal, rs *store.RawBlockStore, probeStart, rangeLo, rangeHi, tip, lowMissing int64, workerID, workers, laneInflight int, inFlight map[int64][32]byte) rawBatchClaim {
	var claim rawBatchClaim
	if probeStart < rangeLo {
		probeStart = rangeLo
	}
	if workers < 1 {
		workers = 1
	}
	batchCap := EffectiveProgressiveBatchSizeForIBD(bs, workers)
	if laneInflight >= batchCap {
		return claim
	}
	maxNew := batchCap - laneInflight
	for probe := probeStart; probe <= rangeHi && probe <= tip && len(claim.heights) < maxNew; probe++ {
		if _, busy := inFlight[probe]; busy {
			if len(claim.heights) > 0 {
				break
			}
			if shouldFillContiguousFrontierFirst(bs, lowMissing) {
				break
			}
			continue
		}
		h80, err := j.ReadHeaderAt(probe)
		if err != nil {
			return claim
		}
		hash := pow.BlockHashLE(h80)
		if store.HasStoredBodyAtHeight(j, rs, probe, bs.chainNet()) {
			if len(claim.heights) > 0 {
				break
			}
			continue
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
	var total int
	var lastErr error
	for batch := 0; batch < maxBatches; batch++ {
		if w != nil && w.PeerAddr != "" {
			s.noteLanePeer(workerID, w.PeerAddr)
		}
		if scorer != nil {
			if stallPeer, stalled := s.maybePenalizeStallingPeer(bs, scorer, book); stalled {
				return total, blockStallError(stallPeer)
			}
			if timeoutPeer, timedOut := s.maybePenalizeDownloadTimeout(bs, scorer, book); timedOut {
				return total, blockDownloadTimeoutError(timeoutPeer)
			}
		}
		claim, ok := s.claimBatch(bs, workerID)
		if !ok {
			break
		}
		if s.syncWorkers > 1 {
			applog.Line("block", fmt.Sprintf("progressive getdata heights %d..%d (%d block(s), lane %d/%d)", claim.lo, claim.hi, len(claim.hashes), workerID, s.syncWorkers))
		} else {
			applog.Line("block", fmt.Sprintf("progressive getdata heights %d..%d (%d block(s))", claim.lo, claim.hi, len(claim.hashes)))
		}
		NoteBlockGetdata(claim.lo, claim.hi, workerID)
		lanes := s.syncWorkerCount()
		batchTimeout := EffectiveBlockDownloadTimeout(bs, lanes)
		batchCtx, endBatch := s.startBatch(workerID, ctx, batchTimeout)
		n, ferr := fetchAndStoreRawBlocksBatch(batchCtx, w, p, claim.hashes, claim.heights, bs, lanes)
		endBatch()
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
		"in_flight_batches": len(s.inFlight),
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
	}
	if !s.lastStoredAt.IsZero() {
		out["last_block_stored_at"] = s.lastStoredAt.Unix()
	}
	lanes := s.syncWorkers
	if lanes < 1 {
		lanes = 1
	}
	out["block_download_timeout_sec"] = int64(BlockDownloadTimeout(lanes-1, 60).Seconds())
	out["block_stalling_timeout_sec"] = int64(blockStallingTimeout.Seconds())
	out["block_stalling_timeout_body_ibd_sec"] = int64(blockStallingTimeoutBodyIBD.Seconds())
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
	out["max_blocks_in_transit_per_peer"] = EffectiveProgressiveBatchSize(lanes)
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
	return out
}
