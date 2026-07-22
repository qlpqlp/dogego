// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
)

var medianTimePastCache struct {
	mu      sync.Mutex
	entries map[int64]int64
}

// ClearMedianTimePastCache drops cached MTP results (tests / chain rewind).
func ClearMedianTimePastCache() {
	medianTimePastCache.mu.Lock()
	medianTimePastCache.entries = nil
	medianTimePastCache.mu.Unlock()
}

// MedianTimePastAt returns the median timestamp of up to 11 blocks ending at prevHeight (Core MTP).
func MedianTimePastAt(j HeaderChain, prevHeight int64) (int64, error) {
	medianTimePastCache.mu.Lock()
	if v, ok := medianTimePastCache.entries[prevHeight]; ok {
		medianTimePastCache.mu.Unlock()
		return v, nil
	}
	medianTimePastCache.mu.Unlock()

	v, err := medianTimePastAtUncached(j, prevHeight)
	if err != nil {
		return 0, err
	}
	medianTimePastCache.mu.Lock()
	if medianTimePastCache.entries == nil {
		medianTimePastCache.entries = make(map[int64]int64)
	}
	medianTimePastCache.entries[prevHeight] = v
	medianTimePastCache.mu.Unlock()
	return v, nil
}

func medianTimePastAtUncached(j HeaderChain, prevHeight int64) (int64, error) {
	if j == nil {
		return 0, fmt.Errorf("nil header journal")
	}
	if prevHeight < 0 {
		h0, err := j.ReadHeaderAt(0)
		if err != nil {
			return 0, err
		}
		return int64(binary.LittleEndian.Uint32(h0[68:72])), nil
	}
	var ts []int64
	for i := 0; i < 11 && prevHeight-int64(i) >= 0; i++ {
		h80, err := j.ReadHeaderAt(prevHeight - int64(i))
		if err != nil {
			return 0, err
		}
		ts = append(ts, int64(binary.LittleEndian.Uint32(h80[68:72])))
	}
	if len(ts) == 0 {
		return 0, fmt.Errorf("no timestamps through height %d", prevHeight)
	}
	sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
	return ts[len(ts)/2], nil
}
