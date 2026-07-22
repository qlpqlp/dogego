// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"fmt"

	"dogego/chain"
	"dogego/wire"
)

// ErrSequenceLock is returned when BIP68 relative lock-times are not satisfied.
var ErrSequenceLock = errors.New("consensus: BIP68 sequence locks not satisfied")

// SequenceEvalBlock is the block context for sequence-lock checks (Core CBlockIndex at eval height).
type SequenceEvalBlock struct {
	Height int64
}

// EnforceBIP68Sequence reports whether tx version and flags require BIP68 sequence locks.
func EnforceBIP68Sequence(tx *wire.Tx) bool {
	if tx == nil {
		return false
	}
	return uint32(tx.Version) >= 2
}

// CalculateSequenceLocks returns the minimum height/time (nLockTime semantics) implied by BIP68 inputs.
func CalculateSequenceLocks(tx *wire.Tx, prevHeights []int, journal HeaderChain, enforce bool) (minHeight int, minTime int64) {
	minHeight = -1
	minTime = -1
	if tx == nil || !enforce || !EnforceBIP68Sequence(tx) {
		return minHeight, minTime
	}
	if len(prevHeights) != len(tx.Vin) {
		return minHeight, minTime
	}
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if in.Sequence&wire.SequenceLocktimeDisableFlag != 0 {
			continue
		}
		coinHeight := prevHeights[i]
		if in.Sequence&wire.SequenceLocktimeTypeFlag != 0 {
			ancestor := int64(coinHeight) - 1
			if ancestor < 0 {
				ancestor = 0
			}
			coinMTP, err := MedianTimePastAt(journal, ancestor)
			if err != nil {
				continue
			}
			lockSec := int64(in.Sequence&wire.SequenceLocktimeMask) << wire.SequenceLocktimeGranularity
			if t := coinMTP + lockSec - 1; t > minTime {
				minTime = t
			}
		} else {
			lockBlocks := int(in.Sequence & wire.SequenceLocktimeMask)
			if h := coinHeight + lockBlocks - 1; h > minHeight {
				minHeight = h
			}
		}
	}
	return minHeight, minTime
}

// EvaluateSequenceLocks reports whether block satisfies calculated sequence lock pair.
func EvaluateSequenceLocks(block SequenceEvalBlock, journal HeaderChain, minHeight int, minTime int64) bool {
	if minHeight < 0 && minTime < 0 {
		return true
	}
	var blockMTP int64
	if journal != nil && block.Height > 0 {
		mtp, err := MedianTimePastAt(journal, block.Height-1)
		if err == nil {
			blockMTP = mtp
		}
	}
	if minHeight >= 0 && int64(minHeight) >= block.Height {
		return false
	}
	if minTime >= 0 && minTime >= blockMTP {
		return false
	}
	return true
}

// SequenceLocks is CalculateSequenceLocks + EvaluateSequenceLocks for block.
func SequenceLocks(tx *wire.Tx, block SequenceEvalBlock, prevHeights []int, journal HeaderChain, enforceCSV bool) bool {
	enforce := EnforceBIP68Sequence(tx) && enforceCSV
	minH, minT := CalculateSequenceLocks(tx, prevHeights, journal, enforce)
	return EvaluateSequenceLocks(block, journal, minH, minT)
}

// SequenceLocksAt applies CSV activation for net at evalBlock.Height.
func SequenceLocksAt(tx *wire.Tx, block SequenceEvalBlock, prevHeights []int, journal HeaderChain, net chain.Network) bool {
	return SequenceLocks(tx, block, prevHeights, journal, CSVActiveAt(block.Height, net))
}

// CheckTxSequenceLocks rejects txs whose BIP68 locks are not satisfied at evalBlock.
func CheckTxSequenceLocks(tx *wire.Tx, evalBlock SequenceEvalBlock, prevHeights []int, journal HeaderChain, net chain.Network) error {
	if tx == nil || IsCoinbaseTx(tx) {
		return nil
	}
	if !SequenceLocksAt(tx, evalBlock, prevHeights, journal, net) {
		return fmt.Errorf("%w", ErrSequenceLock)
	}
	return nil
}
