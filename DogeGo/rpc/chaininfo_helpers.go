// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// ChainIBDSnapshot is Core-shaped sync state (exported for UI / P2P status).
type ChainIBDSnapshot struct {
	Blocks               int64
	Headers              int64
	ContiguousRaw        int64
	IBD                  bool
	VerificationProgress float64
}

// ComputeChainIBDSnapshot returns initialblockdownload / verificationprogress inputs.
func ComputeChainIBDSnapshot(j HeaderJournal, chainName string, raw *store.RawBlockStore, paths *DataPaths) ChainIBDSnapshot {
	s := computeChainIBDState(j, chainName, raw, paths)
	return ChainIBDSnapshot{
		Blocks:               s.blocks,
		Headers:              s.headers,
		ContiguousRaw:        s.contiguousH,
		IBD:                  s.ibd,
		VerificationProgress: s.verProg,
	}
}

type chainIBDState struct {
	blocks, headers, contiguousH int64
	ibd                          bool
	verProg                      float64
}

func ibdTimeParams(paths *DataPaths) (maxTipAge, nowUnix int64) {
	maxTipAge = int64(chain.DefaultMaxTipAge)
	nowUnix = time.Now().Unix()
	if paths != nil {
		if paths.MaxTipAgeSec > 0 {
			maxTipAge = paths.MaxTipAgeSec
		}
		if paths.MedianPeerTimeOffset != nil {
			nowUnix += int64(paths.MedianPeerTimeOffset())
		}
	}
	return maxTipAge, nowUnix
}

// rpcNetworkNowUnix returns Core GetTime for verifychain and header re-checks.
func rpcNetworkNowUnix(paths *DataPaths) int64 {
	_, now := ibdTimeParams(paths)
	return now
}

func computeChainIBDState(j HeaderJournal, chainName string, raw *store.RawBlockStore, paths *DataPaths) chainIBDState {
	blocks, headers, contiguousH := activeChainFromJournal(j, raw, paths)
	if contiguousH < 0 {
		contiguousH = contiguousBodyHeight(j, raw, paths)
	}
	maxTipAge, nowUnix := ibdTimeParams(paths)
	ibd, verProg := ibdProgress(j, chainName, headers, contiguousH, blocks, maxTipAge, nowUnix, paths)
	ibd = applyIBDExitLatch(ibd)
	return chainIBDState{blocks: blocks, headers: headers, contiguousH: contiguousH, ibd: ibd, verProg: verProg}
}

// confirmationsFromTip returns Core-style confirmations when chainActive tip is already known.
func confirmationsFromTip(chainTip, blockHeight int64) int64 {
	if chainTip < 0 || blockHeight < 0 || blockHeight > chainTip {
		return 0
	}
	return chainTip - blockHeight + 1
}

// confirmationsAtChainActive returns Core-style confirmations for a block at blockHeight (0 if ahead of chainActive).
func confirmationsAtChainActive(j HeaderJournal, raw *store.RawBlockStore, blockHeight int64, paths ...*DataPaths) int64 {
	if j == nil {
		return 0
	}
	chainTip, _, _ := activeChainFromJournal(j, raw, paths...)
	return confirmationsFromTip(chainTip, blockHeight)
}

// ActiveChainBlockHeight returns Core chainActive height for UI and height-range checks.
func ActiveChainBlockHeight(j HeaderJournal, raw *store.RawBlockStore, paths ...*DataPaths) int64 {
	if j == nil {
		return -1
	}
	blocks, _, _ := activeChainFromJournal(j, raw, paths...)
	return blocks
}

func corePrunedFromSummary(paths *DataPaths) bool {
	if paths == nil || paths.ChainDataDir == "" {
		return false
	}
	_, ok := store.LoadPruneMarker(paths.ChainDataDir)
	return ok
}

func pruneHeightFromSummary(paths *DataPaths) interface{} {
	if paths == nil || paths.ChainDataDir == "" {
		return nil
	}
	if h, ok := store.LoadPruneMarker(paths.ChainDataDir); ok {
		return h
	}
	return nil
}

func sizeOnDiskNote(paths *DataPaths) string {
	return "headers.bin plus rawblocks/*.bin under the chain datadir"
}

// contiguousBodyHeight returns chainActive body coverage from native rawblocks.
func contiguousBodyHeight(j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths) int64 {
	_, _, cont := activeChainFromJournal(j, raw, paths)
	return cont
}

func ibdProgress(j HeaderJournal, chainName string, headers, bodyCont, blocks int64, maxTipAgeSec int64, nowUnix int64, paths *DataPaths) (ibd bool, verProg float64) {
	verProg = 1.0
	if headers < 0 {
		return false, verProg
	}
	// Headers ahead of chainActive (UTXO/connect tip) - primary IBD signal (Core pindexBestHeader vs chainActive).
	if blocks >= 0 && headers > blocks {
		return true, ConnectedVerificationProgress(headers, blocks)
	}
	want := int(headers) + 1
	have := int(bodyCont) + 1
	if bodyCont < 0 {
		have = 0
	}
	if want > 0 && have < want {
		return true, float64(have) / float64(want)
	}
	// Core IsInitialBlockDownload: remain in IBD until chain work reaches nMinimumChainWork (mainnet 5,050,000).
	if j != nil {
		if min, ok := chain.MinimumChainWorkForRPCChain(chainName); ok && min.Sign() > 0 {
			if cw, ok := chainWorkThrough(j, headers, paths); ok && cw.Cmp(min) < 0 {
				return true, verProg
			}
		}
	}
	// Core IsInitialBlockDownload: chainActive tip block time must be within -maxtipage of adjusted network time.
	if j != nil && blocks >= 0 {
		age := maxTipAgeSec
		if age <= 0 {
			age = chain.DefaultMaxTipAge
		}
		adjNow := nowUnix
		if adjNow <= 0 {
			adjNow = time.Now().Unix()
		}
		if tipTime, err := headerBlockTime(j, blocks); err == nil && tipTime < adjNow-age {
			return true, verProg
		}
	}
	return false, verProg
}

func chainWorkThrough(j HeaderJournal, through int64, paths *DataPaths) (*big.Int, bool) {
	if paths != nil && paths.ChainWorkCacheReady != nil && !paths.ChainWorkCacheReady() && through > 50_000 {
		return nil, false
	}
	if paths != nil && paths.CumulativeChainWork != nil {
		if w, ok := paths.CumulativeChainWork(through); ok {
			return w, true
		}
	}
	w, err := cumulativeChainworkBig(j, through)
	if err != nil {
		return nil, false
	}
	return w, true
}

func headerBlockTime(j HeaderJournal, height int64) (int64, error) {
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint32(h80[68:72])), nil
}

// chainTxCountForVerification returns cumulative tx count for Core GuessVerificationProgress.
func chainTxCountForVerification(j HeaderJournal, txIndex *store.TxIndex, tip int64) int64 {
	if tip < 0 {
		return 0
	}
	if txIndex != nil {
		if n, _, err := txIndex.CachedStats(60 * time.Second); err == nil && n > 0 {
			return int64(n)
		}
	}
	return tip + 1
}

// TxVerificationProgress estimates Core GuessVerificationProgress from tx volume (mainnet curve only).
func TxVerificationProgress(chainName string, j HeaderJournal, txIndex *store.TxIndex, blocks int64) (float64, bool) {
	return txVerificationProgress(chainName, j, txIndex, blocks)
}

func txVerificationProgress(chainName string, j HeaderJournal, txIndex *store.TxIndex, blocks int64) (float64, bool) {
	data, ok := chainTxDataForNetwork(chainName)
	if !ok || j == nil || blocks < 0 {
		return 0, false
	}
	tipTime, err := headerBlockTime(j, blocks)
	if err != nil {
		return 0, false
	}
	txCount := chainTxCountForVerification(j, txIndex, blocks)
	if txCount <= 0 {
		return 0, false
	}
	return guessVerificationProgress(data, txCount, tipTime), true
}

// chainTipAheadStatus is Core getchaintips status for the header journal ahead of chainActive.
func chainTipAheadStatus(blocks, headerTip, contiguousRaw int64, hasRaw bool) string {
	if headerTip <= blocks {
		return ""
	}
	if !hasRaw || contiguousRaw < 0 {
		return "headers-only"
	}
	if contiguousRaw > blocks && contiguousRaw < headerTip {
		return "valid-headers"
	}
	return "headers-only"
}

// headerChainWorkMeetsMinimum reports whether the header journal through height has at least Core nMinimumChainWork.
func headerChainWorkMeetsMinimum(j HeaderJournal, chainName string, through int64) (bool, error) {
	min, ok := chain.MinimumChainWorkForRPCChain(chainName)
	if !ok || min == nil || min.Sign() == 0 {
		return true, nil
	}
	cw, err := cumulativeChainworkBig(j, through)
	if err != nil {
		return false, err
	}
	return cw.Cmp(min) >= 0, nil
}

// minimumChainWorkFields returns Core-shaped diagnostic fields for getblockchaininfo.
func minimumChainWorkFields(j HeaderJournal, chainName string, headerHeight int64, paths *DataPaths) map[string]interface{} {
	min, ok := chain.MinimumChainWorkForRPCChain(chainName)
	if !ok || min == nil || min.Sign() == 0 {
		return nil
	}
	out := map[string]interface{}{
		"dogego_minimum_chain_work": chain.MinimumChainWorkHex(chain.MainnetDogecoin),
	}
	if headerHeight < 0 {
		out["dogego_minimum_chain_work_met"] = false
		return out
	}
	var cw *big.Int
	var err error
	if w, ok := chainWorkThrough(j, headerHeight, paths); ok {
		cw = w
	} else if j == nil {
		out["dogego_minimum_chain_work_met"] = false
		return out
	} else {
		cw, err = cumulativeChainworkBig(j, headerHeight)
		if err != nil {
			out["dogego_minimum_chain_work_met"] = false
			return out
		}
	}
	out["dogego_minimum_chain_work_met"] = cw.Cmp(min) >= 0
	out["dogego_header_chainwork"] = pow.ChainworkHex(cw)
	return out
}

// chainWorkBelowMinimum is used by tests.
func chainWorkBelowMinimum(cw, min *big.Int) bool {
	if min == nil || min.Sign() == 0 {
		return false
	}
	return cw.Cmp(min) < 0
}

// ChainActiveTip returns chainActive height and display hash for RPC waiters and confirmations.
func ChainActiveTip(j HeaderJournal, raw *store.RawBlockStore, paths ...*DataPaths) (height int64, hash string, err error) {
	if j == nil {
		return -1, "", fmt.Errorf("no header journal")
	}
	height, _, _ = activeChainFromJournal(j, raw, paths...)
	hash, err = blockHashHexAt(j, height)
	return height, hash, err
}

// activeChainFromJournal returns Core chainActive-equivalent height, header journal tip, and contiguous raw height.
func activeChainFromJournal(j HeaderJournal, raw *store.RawBlockStore, paths ...*DataPaths) (blocks, headerTip, contiguousRaw int64) {
	if j == nil {
		return 0, 0, -1
	}
	headerTip = -1
	if hj, ok := j.(*store.HeaderJournal); ok {
		if tip, ok := hj.DiskTip(); ok && tip >= 0 {
			headerTip = tip
		}
	}
	if headerTip < 0 {
		var err error
		headerTip, err = j.TipHeight()
		if err != nil {
			return 0, 0, -1
		}
	}
	contiguousRaw = contiguousRawHeight(j, raw, paths...)
	connected := connectedTipFromPaths(paths...)
	blocks = activeChainBlockHeight(headerTip, contiguousRaw, raw != nil, connected)
	return blocks, headerTip, contiguousRaw
}

func connectedTipFromPaths(paths ...*DataPaths) int64 {
	for _, p := range paths {
		if p != nil && p.Utxo != nil {
			return p.Utxo.TipHeight()
		}
	}
	return -1
}

// contiguousRawHeight prefers a node-maintained cache (UI/P2P) to avoid O(tip) genesis scans during IBD.
func contiguousRawHeight(j HeaderJournal, raw *store.RawBlockStore, paths ...*DataPaths) int64 {
	for _, p := range paths {
		if p != nil && p.ContiguousRawHeight != nil {
			if ch := p.ContiguousRawHeight(); ch >= 0 {
				return ch
			}
		}
	}
	if raw == nil {
		return -1
	}
	hj, ok := j.(*store.HeaderJournal)
	if !ok {
		return -1
	}
	tip, err := hj.TipHeight()
	if err != nil || tip < 0 || tip > 50_000 {
		return -1
	}
	ch, err := store.ContiguousRawBodyHeight(hj, raw)
	if err != nil {
		return -1
	}
	return ch
}

// activeChainBlockHeight returns Core getblockchaininfo "blocks" (chainActive: last ConnectBlock height).
// Uses UTXO cache tip when wired; otherwise highest contiguous stored body height (not orphan bodies ahead of connect).
func activeChainBlockHeight(headerTip, contiguousRaw int64, hasRawStore bool, connectedTip int64) int64 {
	if headerTip < 0 {
		return 0
	}
	if !hasRawStore {
		return headerTip
	}
	if connectedTip >= 0 {
		if contiguousRaw >= 0 && connectedTip > contiguousRaw {
			return contiguousRaw
		}
		return connectedTip
	}
	if contiguousRaw >= 0 {
		return contiguousRaw
	}
	return 0
}

// headerDifficultyAt returns difficulty for the header at height (Core chainActive tip fields).
func headerDifficultyAt(j HeaderJournal, height int64) (float64, error) {
	if height < 0 {
		return 0, fmt.Errorf("negative height %d", height)
	}
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return 0, err
	}
	bitsU := binary.LittleEndian.Uint32(h80[72:76])
	return pow.DifficultyFromCompact(bitsU)
}

// tipDifficulty returns the difficulty of the current tip header (same compact nBits → double as getdifficulty).
func tipDifficulty(j HeaderJournal) (float64, error) {
	tip, err := j.TipHeight()
	if err != nil {
		return 0, err
	}
	return headerDifficultyAt(j, tip)
}

// blockHashHexAt returns the display hash of the header at height.
func blockHashHexAt(j HeaderJournal, height int64) (string, error) {
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return "", err
	}
	return pow.BlockHashHex(h80), nil
}

// medianTimePastAfterPrev returns the median of up to 11 timestamps at heights prev, prev-1, … (prev < 0: genesis time only).
// Same window as consensus.medianTimePast(v, prev) for validating the block at height prev+1.
func medianTimePastAfterPrev(j HeaderJournal, prev int64) (int64, error) {
	if prev < 0 {
		h0, err := j.ReadHeaderAt(0)
		if err != nil {
			return 0, err
		}
		return int64(binary.LittleEndian.Uint32(h0[68:72])), nil
	}
	var ts []int64
	for i := 0; i < 11 && prev-int64(i) >= 0; i++ {
		h80, err := j.ReadHeaderAt(prev - int64(i))
		if err != nil {
			return 0, err
		}
		ts = append(ts, int64(binary.LittleEndian.Uint32(h80[68:72])))
	}
	if len(ts) == 0 {
		return 0, fmt.Errorf("no timestamps")
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	return ts[len(ts)/2], nil
}

// tipMedianTimePast returns Core-style "mediantime" for the best header (MTP for a child of the current tip).
func tipMedianTimePast(j HeaderJournal) (int64, error) {
	tip, err := j.TipHeight()
	if err != nil {
		return 0, err
	}
	return medianTimePastAfterPrev(j, tip-1)
}

// headerMedianTimePast returns the MTP Core associates with the header at blockHeight (ancestors only, not that header's time).
func headerMedianTimePast(j HeaderJournal, blockHeight int64) (int64, error) {
	if blockHeight < 0 {
		return 0, fmt.Errorf("negative block height %d", blockHeight)
	}
	return medianTimePastAfterPrev(j, blockHeight-1)
}

// medianPeerTimeOffset returns Core getnetworkinfo/getinfo timeoffset (median of connected peers).
func medianPeerTimeOffset(paths *DataPaths) int32 {
	if paths == nil {
		return 0
	}
	if paths.MedianPeerTimeOffset != nil {
		return paths.MedianPeerTimeOffset()
	}
	if paths.P2PStats == nil {
		return 0
	}
	snap := paths.P2PStats()
	if snap == nil {
		return 0
	}
	if v, ok := snap["median_timeoffset"].(int32); ok {
		return v
	}
	if v, ok := snap["median_timeoffset"].(int); ok {
		return int32(v)
	}
	return 0
}

// mergeDogegoAddrbookFromP2P copies addrbook tried/new counts from a live P2P snapshot.
func mergeDogegoAddrbookFromP2P(res map[string]interface{}, snap map[string]interface{}) {
	if res == nil || snap == nil {
		return
	}
	if v, ok := snap["addrbook_tried"].(int); ok {
		res["dogego_addrbook_tried"] = v
	}
	if v, ok := snap["addrbook_new"].(int); ok {
		res["dogego_addrbook_new"] = v
	}
	if v, ok := snap["addrbook_tried_max"].(int); ok {
		res["dogego_addrbook_tried_max"] = v
	}
	if v, ok := snap["addrbook_new_max"].(int); ok {
		res["dogego_addrbook_new_max"] = v
	}
	if v, ok := snap["addrbook_tried_buckets_used"].(int); ok {
		res["dogego_addrbook_tried_buckets_used"] = v
	}
	if v, ok := snap["addrbook_new_buckets_used"].(int); ok {
		res["dogego_addrbook_new_buckets_used"] = v
	}
	if v, ok := snap["addrbook_tried_bucket_max_fill"].(int); ok {
		res["dogego_addrbook_tried_bucket_max_fill"] = v
	}
	if v, ok := snap["addrbook_new_bucket_max_fill"].(int); ok {
		res["dogego_addrbook_new_bucket_max_fill"] = v
	}
	if v, ok := snap["addrbook_tried_buckets_total"].(int); ok {
		res["dogego_addrbook_tried_buckets_total"] = v
	}
	if v, ok := snap["addrbook_new_buckets_total"].(int); ok {
		res["dogego_addrbook_new_buckets_total"] = v
	}
	if v, ok := snap["addrbook_bucket_slot_cap"].(int); ok {
		res["dogego_addrbook_bucket_slot_cap"] = v
	}
	if v, ok := snap["addrbook_n_key_set"].(bool); ok {
		res["dogego_addrbook_n_key_set"] = v
	}
	mergeDogegoCmpctRelayFromP2P(res, snap)
}

// DogegoCmpctRelayCounterKeys are getblockchaininfo dogego_cmpct_* fields for operator probes.
func DogegoCmpctRelayCounterKeys() []string {
	return []string{
		"dogego_cmpct_in",
		"dogego_cmpct_mempool_hit",
		"dogego_cmpct_getblocktxn_out",
		"dogego_cmpct_blocktxn_in",
		"dogego_cmpct_reconstruct_ok",
		"dogego_cmpct_reconstruct_fail",
		"dogego_cmpct_announced_out",
		"dogego_cmpct_served_getdata",
		"dogego_cmpct_fallback_full_block",
		"dogego_cmpct_blocktxn_served",
		"dogego_cmpct_reconstruct_fallback_getdata",
	}
}

// mergeDogegoCmpctRelayFromP2P copies BIP152 HB and compact-block relay counters from a live P2P snapshot.
func mergeDogegoCmpctRelayFromP2P(res map[string]interface{}, snap map[string]interface{}) {
	if res == nil || snap == nil {
		return
	}
	for _, k := range append([]string{
		"bip152_hb_to", "bip152_hb_from", "bip152_hb_max",
	}, DogegoCmpctRelayCounterKeys()...) {
		if v, ok := snap[k]; ok {
			res[k] = v
		}
	}
	for _, k := range DogegoCmpctRelayCounterKeys() {
		if _, ok := res[k]; !ok {
			res[k] = uint64(0)
		}
	}
}
