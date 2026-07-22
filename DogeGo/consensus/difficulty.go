// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"

	"dogego/pow"
)

// batchView reads header time/bits from the journal plus headers validated earlier in the same batch.
type headerTimeBitsStore interface {
	TipHeight() (int64, error)
	ReadHeaderAt(height int64) ([]byte, error)
}

type batchView struct {
	j     headerTimeBitsStore
	tip0  int64 // journal tip height before this batch
	times []uint32
	bits  []uint32
}

func (v *batchView) timeAt(h int64) (uint32, error) {
	if h <= v.tip0 {
		buf, err := v.j.ReadHeaderAt(h)
		if err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint32(buf[68:72]), nil
	}
	i := int(h - v.tip0 - 1)
	if i < 0 || i >= len(v.times) {
		return 0, fmt.Errorf("timeAt height %d oob (batch len %d)", h, len(v.times))
	}
	return v.times[i], nil
}

func (v *batchView) bitsAt(h int64) (uint32, error) {
	if h <= v.tip0 {
		buf, err := v.j.ReadHeaderAt(h)
		if err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint32(buf[72:76]), nil
	}
	i := int(h - v.tip0 - 1)
	if i < 0 || i >= len(v.bits) {
		return 0, fmt.Errorf("bitsAt height %d oob", h)
	}
	return v.bits[i], nil
}

func medianTimePast(v *batchView, prevHeight int64) (int64, error) {
	var ts []int64
	for i := 0; i < 11 && prevHeight-int64(i) >= 0; i++ {
		t, err := v.timeAt(prevHeight - int64(i))
		if err != nil {
			return 0, err
		}
		ts = append(ts, int64(t))
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	return ts[len(ts)/2], nil
}

func allowDigishieldMinDifficultyForBlock(v *batchView, prevHeight int64, candTime uint32, dc DogeConsensus) (bool, error) {
	if !dc.PowAllowDigishieldMinDifficultyBlocks {
		return false, nil
	}
	if dc.EnforceStrictMinDifficulty {
		powLim := pow.DogePowLimitCompact()
		prevBits, err := v.bitsAt(prevHeight)
		if err != nil {
			return false, err
		}
		if prevBits == powLim {
			return false, nil
		}
		mtp, err := medianTimePast(v, prevHeight)
		if err != nil {
			return false, err
		}
		prevT, err := v.timeAt(prevHeight)
		if err != nil {
			return false, err
		}
		if int64(candTime) <= mtp+dc.PowTargetSpacing*10 {
			return false, nil
		}
		return int64(candTime) > int64(prevT)+dc.PowTargetSpacing*10, nil
	}
	prevT, err := v.timeAt(prevHeight)
	if err != nil {
		return false, err
	}
	return int64(candTime) > int64(prevT)+dc.PowTargetSpacing*2, nil
}

func allowMinDifficultyForBlock(v *batchView, prevHeight int64, candTime uint32, dc DogeConsensus) (bool, error) {
	if !dc.PowAllowMinDifficultyBlocks {
		return false, nil
	}
	prevT, err := v.timeAt(prevHeight)
	if err != nil {
		return false, err
	}
	return int64(candTime) > int64(prevT)+dc.PowTargetSpacing*2, nil
}

func calculateDogecoinNextWorkRequired(v *batchView, prevHeight int64, firstBlockTime int64, dc DogeConsensus) (uint32, error) {
	nHeight := prevHeight + 1
	retargetSpan := dc.PowTargetTimespan
	actual := int64(0)
	lastT, err := v.timeAt(prevHeight)
	if err != nil {
		return 0, err
	}
	actual = int64(lastT) - firstBlockTime
	modulated := actual
	var minSpan, maxSpan int64
	if dc.Digishield {
		modulated = retargetSpan + (modulated-retargetSpan)/8
		minSpan = retargetSpan - (retargetSpan / 4)
		maxSpan = retargetSpan + (retargetSpan / 2)
	} else if nHeight > 10000 {
		minSpan = retargetSpan / 4
		maxSpan = retargetSpan * 4
	} else if nHeight > 5000 {
		minSpan = retargetSpan / 8
		maxSpan = retargetSpan * 4
	} else {
		minSpan = retargetSpan / 16
		maxSpan = retargetSpan * 4
	}
	if modulated < minSpan {
		modulated = minSpan
	}
	if modulated > maxSpan {
		modulated = maxSpan
	}
	prevBits, err := v.bitsAt(prevHeight)
	if err != nil {
		return 0, err
	}
	bnNew, err := pow.TargetFromCompact(prevBits)
	if err != nil {
		return 0, err
	}
	bnNew.Mul(bnNew, big.NewInt(modulated))
	bnNew.Div(bnNew, big.NewInt(retargetSpan))
	limit, _ := new(big.Int).SetString(PowLimitHex, 16)
	if bnNew.Cmp(limit) > 0 {
		bnNew.Set(limit)
	}
	return pow.CompactFromBigInt(bnNew), nil
}

func getNextWorkRequired(v *batchView, prevHeight int64, candTime uint32, dc DogeConsensus) (uint32, error) {
	powLim := pow.DogePowLimitCompact()
	if prevHeight < 0 {
		return 0, fmt.Errorf("invalid prev height")
	}
	if ok, err := allowDigishieldMinDifficultyForBlock(v, prevHeight, candTime, dc); err != nil {
		return 0, err
	} else if ok {
		return powLim, nil
	}
	fNewDifficultyProtocol := dc.Digishield
	difficultyAdjustmentInterval := int64(1)
	if !fNewDifficultyProtocol {
		difficultyAdjustmentInterval = dc.PowTargetTimespan / dc.PowTargetSpacing
	}
	if (prevHeight+1)%difficultyAdjustmentInterval != 0 {
		if dc.PowAllowMinDifficultyBlocks {
			prevT, err := v.timeAt(prevHeight)
			if err != nil {
				return 0, err
			}
			if int64(candTime) > int64(prevT)+dc.PowTargetSpacing*2 {
				return powLim, nil
			}
			pindex := prevHeight
			interval := dc.PowTargetTimespan / dc.PowTargetSpacing
			if interval < 1 {
				interval = 1
			}
			for pindex > 0 && pindex%interval != 0 {
				b, err := v.bitsAt(pindex)
				if err != nil {
					return 0, err
				}
				if b != powLim {
					break
				}
				pindex--
			}
			b, err := v.bitsAt(pindex)
			if err != nil {
				return 0, err
			}
			return b, nil
		}
		b, err := v.bitsAt(prevHeight)
		return b, err
	}
	blockstogoback := difficultyAdjustmentInterval - 1
	if (prevHeight + 1) != difficultyAdjustmentInterval {
		blockstogoback = difficultyAdjustmentInterval
	}
	nHeightFirst := prevHeight - blockstogoback
	if nHeightFirst < 0 {
		return 0, fmt.Errorf("retarget before genesis")
	}
	firstT, err := v.timeAt(nHeightFirst)
	if err != nil {
		return 0, err
	}
	return calculateDogecoinNextWorkRequired(v, prevHeight, int64(firstT), dc)
}
