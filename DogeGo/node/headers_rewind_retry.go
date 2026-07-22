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
	"dogego/store"
)

// IsHeaderRewindRetryErr reports a local journal rewind that should retry getheaders on the same peer.
func IsHeaderRewindRetryErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "rewound journal") && strings.Contains(s, "retry getheaders")
}

// headerRewindRetryBudget limits how many automatic deep rewinds we attempt before giving up.
const headerRewindRetryBudget = 32
const badNBitsGenesisResetTipCeiling int64 = 500_000

// badNBitsRecoveryDecision reports Core-style recovery after repeated bad-nBits at the same tip.
// At low tip (<500k) DogeGo resets to genesis; at mainnet scale it stops rewinding and forces peer rotation.
func badNBitsRecoveryDecision(tip int64, repeatCount int) (genesisReset, peerRotationOnly bool) {
	if repeatCount < 3 {
		return false, false
	}
	if tip < badNBitsGenesisResetTipCeiling {
		return true, false
	}
	return false, true
}

// maybeRewindOnBadNBits truncates to the previous difficulty period when validation fails on nBits mismatch.
// When the same rewind height would repeat, walks back additional retarget windows (stale compressed times).
func maybeRewindOnBadNBits(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx, validateErr error) (bool, error) {
	if validateErr == nil || !strings.Contains(validateErr.Error(), "bad nBits") {
		return false, nil
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return false, validateErr
	}
	dc := consensus.LookupConsensus(p.Net, tip)
	interval := int64(dc.DifficultyAdjustmentBlocks())
	if interval <= 0 {
		return false, validateErr
	}
	if bs == nil || bs.lastBadNBitsRewind < 0 {
		if rewound, rerr := maybeRewindCompressedHeaderPeriod(j, aux, p, bs); rewound {
			return true, rerr
		}
	}
	rewindTo := headerRewindHeightBeforeRetarget(tip, interval)
	if bs != nil && bs.lastBadNBitsRewind >= 0 && rewindTo >= bs.lastBadNBitsRewind {
		for extra := 0; extra < headerRewindRetryBudget && rewindTo >= bs.lastBadNBitsRewind; extra++ {
			next := rewindTo - interval
			if next < 0 {
				rewindTo = 0
				break
			}
			rewindTo = next
		}
	}
	if rewindTo >= tip {
		return false, validateErr
	}
	if bs != nil {
		if bs.badNBitsRepeatHeight == tip {
			bs.badNBitsRepeatCount++
		} else {
			bs.badNBitsRepeatHeight = tip
			bs.badNBitsRepeatCount = 1
		}
		if bs.badNBitsRepeatCount >= 3 {
			genesisReset, peerRotationOnly := badNBitsRecoveryDecision(tip, bs.badNBitsRepeatCount)
			if genesisReset {
				applog.Line("headers", fmt.Sprintf("bad nBits persists near height %d after %d retries; resetting headers.bin to genesis for clean Core-style IBD restart", tip, bs.badNBitsRepeatCount))
				if err := truncateChainToHeightLocked(j, aux, bs, 0); err != nil {
					return false, err
				}
				if bs.ChainWork != nil {
					bs.ChainWork.Invalidate()
					bs.ChainWork.Warm(j)
				}
				bs.badNBitsRepeatHeight = -1
				bs.badNBitsRepeatCount = 0
				bs.lastBadNBitsRewind = -1
				return true, fmt.Errorf("headers: rewound journal to height 0 after repeated bad nBits (retry getheaders)")
			}
			if peerRotationOnly {
				return false, fmt.Errorf("header sync bad nBits persists near height %d after %d rewind attempt(s); forcing peer rotation without further rewind", tip, bs.badNBitsRepeatCount)
			}
		}
	}
	if shouldDeferHeaderTipTruncateDuringBodyIBD(bs, tip, rewindTo) {
		applog.Line("headers", fmt.Sprintf("bad nBits at height %d - deferring header truncate to %d during forward block IBD (contiguous bodies %d)", tip, rewindTo, bs.ContiguousRawHeight()))
		return false, validateErr
	}
	applog.Line("headers", fmt.Sprintf("bad nBits near tip height %d - rewinding journal to height %d and pruning damaged blocks/index", tip, rewindTo))
	if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
		return false, validateErr
	}
	newTip, err := j.TipHeight()
	if err != nil {
		return false, err
	}
	if newTip > rewindTo {
		applog.Line("headers", fmt.Sprintf("bad nBits rewind did not persist (tip still %d, wanted ≤%d); resetting to genesis for clean IBD", newTip, rewindTo))
		if err := truncateChainToHeightLocked(j, aux, bs, 0); err != nil {
			return false, validateErr
		}
		if bs != nil {
			bs.badNBitsRepeatHeight = -1
			bs.badNBitsRepeatCount = 0
			bs.lastBadNBitsRewind = -1
			if bs.ChainWork != nil {
				bs.ChainWork.Invalidate()
				bs.ChainWork.Warm(j)
			}
		}
		return true, fmt.Errorf("headers: rewound journal to height 0 after bad nBits rewind failed to persist (retry getheaders)")
	}
	if bs != nil {
		bs.lastBadNBitsRewind = rewindTo
		if bs.ChainWork != nil {
			bs.ChainWork.Invalidate()
			bs.ChainWork.Warm(j)
		}
	}
	applog.Line("headers", fmt.Sprintf("header journal truncated to height %d after bad nBits (retry getheaders)", rewindTo))
	return true, fmt.Errorf("headers: rewound journal to height %d after bad nBits (retry getheaders)", rewindTo)
}

func isCheckpointMismatchErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "checkpoint hash mismatch")
}

// parseCheckpointMismatchHeight extracts the chain height from a checkpoint validation error.
func parseCheckpointMismatchHeight(err error) (int64, bool) {
	if err == nil {
		return 0, false
	}
	s := err.Error()
	for _, prefix := range []string{"header at height ", "chain height "} {
		i := strings.Index(s, prefix)
		if i < 0 {
			continue
		}
		rest := s[i+len(prefix):]
		var h int64
		if _, scanErr := fmt.Sscanf(rest, "%d", &h); scanErr == nil && h >= 0 {
			return h, true
		}
	}
	return 0, false
}

// maybeRewindOnCheckpointMismatch truncates when headers.bin has a wrong hash at a Core checkpoint height.
func maybeRewindOnCheckpointMismatch(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx, validateErr error) (bool, error) {
	if !isCheckpointMismatchErr(validateErr) {
		return false, validateErr
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return false, validateErr
	}
	mismatchH, ok := parseCheckpointMismatchHeight(validateErr)
	if !ok {
		mismatchH = tip
	}
	rewindTo := mismatchH - 1
	if rewindTo < 0 {
		rewindTo = 0
	}
	if rewindTo >= tip {
		return false, validateErr
	}
	if shouldDeferHeaderTipTruncateDuringBodyIBD(bs, tip, rewindTo) {
		applog.Line("headers", fmt.Sprintf("checkpoint mismatch at height %d - deferring header truncate during forward block IBD (contiguous bodies %d)", mismatchH, bs.ContiguousRawHeight()))
		return false, validateErr
	}
	applog.Line("headers", fmt.Sprintf("checkpoint hash mismatch at height %d - rewinding journal to height %d (was %d)", mismatchH, rewindTo, tip))
	if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
		return false, err
	}
	if bs != nil && bs.ChainWork != nil {
		bs.ChainWork.Invalidate()
		bs.ChainWork.Warm(j)
	}
	return true, fmt.Errorf("headers: rewound journal to height %d after checkpoint mismatch (retry getheaders)", rewindTo)
}

func isObsoleteAuxParentChainGateErr(err error) bool {
	// Pre-Core-parity builds rejected any non-zero parent chain id; not a local journal defect.
	return err != nil && strings.Contains(err.Error(), "chain id must be zero (litecoin merge-mining parent)")
}

func isAuxpowValidationErr(err error) bool {
	if err == nil || isAuxpowRuleMismatchErr(err) || isObsoleteAuxParentChainGateErr(err) {
		return false
	}
	s := err.Error()
	return strings.Contains(s, " aux:") ||
		strings.Contains(s, "aux hash block mismatch") ||
		strings.Contains(s, "aux parent") ||
		strings.Contains(s, ": missing auxpow")
}

// parseValidateHeaderIndex extracts the batch index from ValidateHeaders error text.
func parseValidateHeaderIndex(err error) (int64, bool) {
	if err == nil {
		return 0, false
	}
	s := err.Error()
	var idx int64
	if _, scanErr := fmt.Sscanf(s, "header %d aux", &idx); scanErr == nil {
		return idx, true
	}
	if _, scanErr := fmt.Sscanf(s, "header %d:", &idx); scanErr == nil {
		return idx, true
	}
	if i := strings.Index(s, "header batch index "); i >= 0 {
		if _, scanErr := fmt.Sscanf(s[i:], "header batch index %d", &idx); scanErr == nil {
			return idx, true
		}
	}
	return 0, false
}

// maybeRewindOnInvalidAuxPow truncates before the auxpow fork when the first header of a batch
// fails aux checks (local/journal damage). Mid-batch failures are left to peer rotation.
func maybeRewindOnInvalidAuxPow(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx, validateErr error) (bool, error) {
	if !isAuxpowValidationErr(validateErr) {
		return false, nil
	}
	activation := consensus.AuxpowActivationHeight(p.Net)
	if activation <= 0 {
		return false, validateErr
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return false, validateErr
	}
	if batchIdx, ok := parseValidateHeaderIndex(validateErr); ok && batchIdx > 0 {
		return false, validateErr
	}
	rewindTo := activation - 1
	if rewindTo < 0 {
		rewindTo = 0
	}
	if tip < activation {
		dc := consensus.LookupConsensus(p.Net, tip)
		interval := int64(dc.DifficultyAdjustmentBlocks())
		if interval > 0 {
			rewindTo = headerRewindHeightBeforeRetarget(tip, interval)
		}
		if rewindTo >= tip {
			rewindTo = 0
		}
		applog.Line("headers", fmt.Sprintf("invalid auxpow before activation at tip %d - rewinding journal to height %d", tip, rewindTo))
	} else {
		if tip <= rewindTo {
			return false, validateErr
		}
		applog.Line("headers", fmt.Sprintf("invalid auxpow at merge-mining boundary (tip %d) - rewinding journal to height %d and clearing aux index", tip, rewindTo))
	}
	if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
		return false, err
	}
	if bs != nil && bs.ChainWork != nil {
		bs.ChainWork.Invalidate()
		bs.ChainWork.Warm(j)
	}
	return true, fmt.Errorf("headers: rewound journal to height %d after invalid auxpow (retry getheaders)", rewindTo)
}

func isAuxpowRuleMismatchErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "legacy scrypt header after auxpow") ||
		strings.Contains(s, "legacy block after auxpow") ||
		strings.Contains(s, "auxpow header before activation") ||
		strings.Contains(s, "auxpow before activation")
}

// maybeRewindOnAuxpowRuleMismatch truncates when the local header journal is past the auxpow fork
// but contains legacy/auxpow headers inconsistent with Core rules (wrong-network or damaged headers.bin).
// When the local tip is still below activation, the mismatch is from the peer batch - try another peer.
func maybeRewindOnAuxpowRuleMismatch(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx, validateErr error) (bool, error) {
	if !isAuxpowRuleMismatchErr(validateErr) {
		return false, nil
	}
	activation := consensus.AuxpowActivationHeight(p.Net)
	if activation <= 0 {
		return false, validateErr
	}
	tip, err := j.TipHeight()
	if err != nil || tip < activation {
		return false, validateErr
	}
	rewindTo := activation - 1
	if rewindTo < 0 {
		rewindTo = 0
	}
	if rewindTo >= tip {
		return false, validateErr
	}
	applog.Line("headers", fmt.Sprintf("auxpow rule mismatch at tip height %d - rewinding journal to height %d (last legacy height before merge-mining)", tip, rewindTo))
	if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
		return false, err
	}
	if bs != nil && bs.ChainWork != nil {
		bs.ChainWork.Invalidate()
		bs.ChainWork.Warm(j)
	}
	return true, fmt.Errorf("headers: rewound journal to height %d after auxpow/legacy mismatch (retry getheaders)", rewindTo)
}

// shouldAttemptDeepHeaderRewind gates last-resort period rewinds to validation-corruption classes only.
// Network stalls/timeouts should rotate peers, not mutate local headers.bin.
func shouldAttemptDeepHeaderRewind(lastErr error) bool {
	if lastErr == nil {
		return false
	}
	if isCheckpointMismatchErr(lastErr) || isAuxpowValidationErr(lastErr) || isAuxpowRuleMismatchErr(lastErr) {
		return true
	}
	s := lastErr.Error()
	return strings.Contains(s, "bad nBits") ||
		strings.Contains(s, "legacy scrypt header after auxpow") ||
		strings.Contains(s, "auxpow header before activation")
}

// runLocalHeaderJournalRecovery tries automatic fixes when every peer hit the same local stale headers.
// lastErr may be the last header-sync failure (auxpow boundary, bad nBits, etc.) to pick a targeted rewind.
func runLocalHeaderJournalRecovery(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx, lastErr error) (bool, error) {
	var rewound bool
	err := withHeaderChainWriteErr(func() error {
		var innerErr error
		rewound, innerErr = runLocalHeaderJournalRecoveryLocked(j, aux, p, bs, lastErr)
		return innerErr
	})
	return rewound, err
}

func parseStoredHeaderHeight(err error) (int64, bool) {
	if err == nil {
		return 0, false
	}
	s := err.Error()
	var h int64
	if _, scanErr := fmt.Sscanf(s, "height %d:", &h); scanErr == nil && h >= 0 {
		return h, true
	}
	return 0, false
}

func runLocalHeaderJournalRecoveryLocked(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx, lastErr error) (bool, error) {
	if j == nil {
		return false, nil
	}
	if lastErr != nil && strings.Contains(lastErr.Error(), "bad prev") {
		tip, err := j.TipHeight()
		if err == nil && tip > 0 {
			rewindTo := int64(0)
			if tip < badNBitsGenesisResetTipCeiling {
				applog.Line("headers", fmt.Sprintf("header recovery: bad prev linkage at tip %d - resetting to genesis for clean IBD", tip))
			} else if h, ok := parseStoredHeaderHeight(lastErr); ok && h > 0 {
				rewindTo = h - 1
			}
			if rewindTo < tip {
				applog.Line("headers", fmt.Sprintf("header recovery: bad prev linkage - rewinding journal to height %d (was %d)", rewindTo, tip))
				if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
					return false, err
				}
				if bs != nil && bs.ChainWork != nil {
					bs.ChainWork.Invalidate()
					bs.ChainWork.Warm(j)
				}
				return true, fmt.Errorf("headers: rewound journal to height %d after bad prev (retry getheaders)", rewindTo)
			}
		}
	}
	if isCheckpointMismatchErr(lastErr) {
		if mismatchH, ok := parseCheckpointMismatchHeight(lastErr); ok {
			tip, err := j.TipHeight()
			if err == nil && tip >= mismatchH {
				rewindTo := mismatchH - 1
				if rewindTo < 0 {
					rewindTo = 0
				}
				if rewindTo < tip {
					applog.Line("headers", fmt.Sprintf("header recovery: checkpoint mismatch - rewinding journal to height %d (was %d)", rewindTo, tip))
					if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
						return false, err
					}
					if bs != nil && bs.ChainWork != nil {
						bs.ChainWork.Invalidate()
						bs.ChainWork.Warm(j)
					}
					return true, fmt.Errorf("headers: rewound journal to height %d after checkpoint mismatch (retry getheaders)", rewindTo)
				}
			}
		}
	}
	if isAuxpowValidationErr(lastErr) {
		if ok, rerr := maybeRewindOnInvalidAuxPow(j, aux, p, bs, lastErr); ok {
			return true, rerr
		}
	}
	if isAuxpowRuleMismatchErr(lastErr) {
		tip, err := j.TipHeight()
		if err == nil && tip >= 0 {
			activation := consensus.AuxpowActivationHeight(p.Net)
			if activation > 0 && tip >= activation {
				rewindTo := activation - 1
				if rewindTo < tip {
					applog.Line("headers", fmt.Sprintf("header recovery: auxpow mismatch - rewinding journal to height %d (was %d)", rewindTo, tip))
					if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
						return false, err
					}
					if bs != nil && bs.ChainWork != nil {
						bs.ChainWork.Invalidate()
						bs.ChainWork.Warm(j)
					}
					return true, fmt.Errorf("headers: rewound journal to height %d after auxpow/legacy mismatch (retry getheaders)", rewindTo)
				}
			}
		}
	}
	rewound, err := maybeRewindCompressedHeaderPeriod(j, aux, p, bs)
	if err != nil {
		return false, err
	}
	if rewound {
		if bs != nil {
			bs.lastBadNBitsRewind = -1
		}
		return true, err
	}
	if !shouldAttemptDeepHeaderRewind(lastErr) {
		return false, nil
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return false, nil
	}
	dc := consensus.LookupConsensus(p.Net, tip)
	interval := int64(dc.DifficultyAdjustmentBlocks())
	if interval <= 0 {
		return false, nil
	}
	rewindTo := headerRewindHeightBeforeRetarget(tip, interval)
	if bs != nil && bs.lastBadNBitsRewind >= 0 && rewindTo >= bs.lastBadNBitsRewind {
		rewindTo -= interval
	}
	if rewindTo < 0 {
		rewindTo = 0
	}
	if rewindTo >= tip {
		return false, nil
	}
	if shouldDeferHeaderTipTruncateDuringBodyIBD(bs, tip, rewindTo) {
		applog.Line("headers", fmt.Sprintf("header sync recovery: defer deep rewind to %d during forward block IBD (tip %d, contiguous bodies %d)", rewindTo, tip, bs.ContiguousRawHeight()))
		return false, nil
	}
	applog.Line("headers", fmt.Sprintf("header sync recovery: deep rewind to height %d (was %d) before retrying peers", rewindTo, tip))
	if err := truncateChainToHeightLocked(j, aux, bs, rewindTo); err != nil {
		return false, err
	}
	if bs != nil {
		bs.lastBadNBitsRewind = rewindTo
		if bs.ChainWork != nil {
			bs.ChainWork.Invalidate()
			bs.ChainWork.Warm(j)
		}
	}
	return true, nil
}

func shouldAutoRecoverHeaderSync(err error) bool {
	if err == nil {
		return false
	}
	if IsHeaderRewindRetryErr(err) {
		return true
	}
	if isAuxpowRuleMismatchErr(err) || isAuxpowValidationErr(err) || isCheckpointMismatchErr(err) || strings.Contains(err.Error(), "bad nBits") {
		return true
	}
	// Keep the node up when discovery/probe fails transiently (DNS blip, firewall, restart).
	s := err.Error()
	if strings.Contains(s, "no peer candidates") ||
		strings.Contains(s, "no peer handshakes succeeded") ||
		strings.Contains(s, "no peers after header recovery") ||
		strings.Contains(s, "no peer candidates for header recovery") {
		return true
	}
	// Keep the node up when every probed peer dropped (Windows wsasend, timeout, wrong fork header, …).
	return recoverableHeaderPeerErr(err)
}
