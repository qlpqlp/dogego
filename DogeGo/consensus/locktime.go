// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"dogego/wire"
)

// ErrNonFinalTx is returned when a transaction is not final for the next block (Core non-final).
var ErrNonFinalTx = errors.New("consensus: non-final transaction")

// IsFinalTx reports whether tx may be included in a block at blockHeight with locktime cutoff blockTime.
// Matches Core validation.cpp IsFinalTx.
func IsFinalTx(tx *wire.Tx, blockHeight int64, blockTime int64) bool {
	if tx == nil || tx.LockTime == 0 {
		return true
	}
	cmp := blockHeight
	if int64(tx.LockTime) >= wire.LocktimeThreshold {
		cmp = blockTime
	}
	if int64(tx.LockTime) < cmp {
		return true
	}
	for i := range tx.Vin {
		if tx.Vin[i].Sequence != wire.SequenceFinal {
			return false
		}
	}
	return true
}

// LockTimeContext holds height/time used to test finality (next-block context for mempool).
type LockTimeContext struct {
	BlockHeight int64
	BlockTime   int64
}

// LockTimeContextForNextBlock builds Core CheckFinalTx context (chain tip + 1, MTP of tip).
func LockTimeContextForNextBlock(j HeaderChain, useMTP bool) (LockTimeContext, error) {
	if j == nil {
		return LockTimeContext{BlockHeight: 0, BlockTime: time.Now().Unix()}, nil
	}
	tip, err := j.TipHeight()
	if err != nil {
		return LockTimeContext{}, err
	}
	ctx := LockTimeContext{BlockHeight: tip + 1}
	if useMTP && tip >= 0 {
		mtp, err := MedianTimePastAt(j, tip)
		if err != nil {
			return LockTimeContext{}, err
		}
		ctx.BlockTime = mtp
		return ctx, nil
	}
	ctx.BlockTime = time.Now().Unix()
	return ctx, nil
}

// LockTimeContextAtBlock builds finality context for connecting block at height (Core ConnectBlock).
func LockTimeContextAtBlock(j HeaderChain, blockHeight int64, useMTP bool) (LockTimeContext, error) {
	ctx := LockTimeContext{BlockHeight: blockHeight}
	if useMTP && j != nil && blockHeight > 0 {
		mtp, err := MedianTimePastAt(j, blockHeight-1)
		if err != nil {
			return LockTimeContext{}, err
		}
		ctx.BlockTime = mtp
		return ctx, nil
	}
	if j != nil && blockHeight >= 0 {
		h80, err := j.ReadHeaderAt(blockHeight)
		if err == nil && len(h80) >= 72 {
			ctx.BlockTime = int64(binary.LittleEndian.Uint32(h80[68:72]))
			return ctx, nil
		}
	}
	ctx.BlockTime = time.Now().Unix()
	return ctx, nil
}

// CheckTxFinal rejects non-final transactions for the given context.
func CheckTxFinal(tx *wire.Tx, ctx LockTimeContext) error {
	if tx == nil || IsFinalTx(tx, ctx.BlockHeight, ctx.BlockTime) {
		return nil
	}
	return fmt.Errorf("%w (nLockTime=%d)", ErrNonFinalTx, tx.LockTime)
}
