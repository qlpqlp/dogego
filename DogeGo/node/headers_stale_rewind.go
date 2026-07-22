// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"fmt"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/store"
	"dogego/wire"
)

// difficultyPeriodMinTimeSpan is the minimum nTime span expected across a full retarget window on a live chain.
func difficultyPeriodMinTimeSpan(dc consensus.DogeConsensus) int64 {
	interval := int64(dc.DifficultyAdjustmentBlocks())
	spacing := dc.PowTargetSpacing
	if interval <= 0 || spacing <= 0 {
		return 0
	}
	const minHours = 16
	span := interval * spacing * minHours
	if floor := interval * spacing * 4; span < floor {
		span = floor
	}
	return span
}

// legacyDifficultyPeriodBlocks is the pre-DigiShield retarget window; used when interval==1 so
// recovery rewinds a meaningful span instead of a single block (which would loop forever).
const legacyDifficultyPeriodBlocks int64 = 2016

func headerRewindHeightBeforeRetarget(tip, interval int64) int64 {
	if interval <= 1 {
		rewindTo := tip - legacyDifficultyPeriodBlocks
		if rewindTo < 0 {
			rewindTo = 0
		}
		return rewindTo
	}
	rewindTo := (tip / interval) * interval
	if rewindTo >= tip {
		rewindTo -= interval
	}
	if rewindTo < 0 {
		rewindTo = 0
	}
	return rewindTo
}

// maybeRewindStaleHeaderTimes truncates the journal to the last difficulty period when an
// incoming headers batch's first timestamp is far ahead of the local tip (common after force-kill
// or a partial sync with compressed times). The caller should retry getheaders from the new tip.
func maybeRewindStaleHeaderTimes(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, decoded []wire.DecodedHeader, bs *BlockStoreCtx) (bool, error) {
	if j == nil || len(decoded) == 0 {
		return false, nil
	}
	tip, err := j.TipHeight()
	if err != nil || tip <= 0 {
		return false, err
	}
	tip80, err := j.ReadHeaderAt(tip)
	if err != nil {
		return false, err
	}
	tipTime := binary.LittleEndian.Uint32(tip80[68:72])
	firstTime := binary.LittleEndian.Uint32(decoded[0].Header80[68:72])
	if firstTime <= tipTime {
		return false, nil
	}
	gap := int64(firstTime) - int64(tipTime)
	dc := consensus.LookupConsensus(p.Net, tip)
	interval := int64(dc.DifficultyAdjustmentBlocks())
	maxGap := interval * dc.PowTargetSpacing * 2
	if maxGap < dc.PowTargetSpacing*4 {
		maxGap = dc.PowTargetSpacing * 4
	}
	// Post-DigiShield retargets every block (interval=1), which would cap maxGap at ~120s and
	// falsely rewind on normal live-chain batches (~minutes ahead of local nTime).
	const minStaleGapSec int64 = 3600
	if maxGap < minStaleGapSec {
		maxGap = minStaleGapSec
	}
	if gap <= maxGap {
		return false, nil
	}
	rewindTo := headerRewindHeightBeforeRetarget(tip, interval)
	if shouldDeferHeaderTipTruncateDuringBodyIBD(bs, tip, rewindTo) {
		return false, nil
	}
	applog.Line("headers", fmt.Sprintf("stale header times: peer first nTime +%ds ahead of local tip height %d - rewinding journal to height %d before refetch",
		gap, tip, rewindTo))
	if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
		return false, err
	}
	return true, fmt.Errorf("headers: rewound journal to height %d (peer timestamps +%ds ahead of local tip; retry getheaders)", rewindTo, gap)
}

// maybeRewindCompressedHeaderPeriod truncates when the active difficulty window has unrealistically
// tight nTime spacing (partial sync with ~60s steps). Catches mainnet bad nBits at retarget before peers respond.
func maybeRewindCompressedHeaderPeriod(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx) (bool, error) {
	if j == nil {
		return false, nil
	}
	tip, err := j.TipHeight()
	if err != nil || tip <= 0 {
		return false, err
	}
	dc := consensus.LookupConsensus(p.Net, tip)
	interval := int64(dc.DifficultyAdjustmentBlocks())
	if tip < interval {
		return false, nil
	}
	periodStart := (tip / interval) * interval
	blocksInPeriod := tip - periodStart + 1
	// Full retarget window, or enough of the active period to judge compressed timestamps
	// (mainnet bad nBits at ~4080 often follows a partial fast-sync period ending at ~4000).
	const minBlocksForPeriodSpanCheck int64 = 32
	if blocksInPeriod < minBlocksForPeriodSpanCheck {
		return false, nil
	}
	start80, err := j.ReadHeaderAt(periodStart)
	if err != nil {
		return false, err
	}
	tip80, err := j.ReadHeaderAt(tip)
	if err != nil {
		return false, err
	}
	startTime := binary.LittleEndian.Uint32(start80[68:72])
	tipTime := binary.LittleEndian.Uint32(tip80[68:72])
	if tipTime <= startTime {
		return false, nil
	}
	span := int64(tipTime) - int64(startTime)
	minSpan := difficultyPeriodMinTimeSpan(dc)
	if minSpan > 0 && blocksInPeriod < interval {
		minSpan = minSpan * blocksInPeriod / interval
		if minSpan < dc.PowTargetSpacing*4 {
			minSpan = dc.PowTargetSpacing * 4
		}
	}
	if minSpan <= 0 || span >= minSpan {
		return false, nil
	}
	rewindTo := headerRewindHeightBeforeRetarget(tip, interval)
	if shouldDeferHeaderTipTruncateDuringBodyIBD(bs, tip, rewindTo) {
		return false, nil
	}
	applog.Line("headers", fmt.Sprintf("compressed header times: difficulty period %d..%d spans %ds (< %ds) - rewinding journal to height %d before refetch",
		periodStart, tip, span, minSpan, rewindTo))
	if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
		return false, err
	}
	return true, fmt.Errorf("headers: rewound journal to height %d (compressed header times in period %d..%d; retry getheaders)", rewindTo, periodStart, tip)
}

