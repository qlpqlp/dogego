// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "dogego/chain"

// LowestMissingSearchStart returns the first height to scan for a missing raw block when
// bodies are known contiguous through contiguousHeight (-1 scans from genesis).
func LowestMissingSearchStart(j *HeaderJournal, raw *RawBlockStore, contiguousHeight int64, net chain.Network) int64 {
	if j == nil || raw == nil || contiguousHeight < 0 {
		return 0
	}
	if HasStoredBodyAtHeight(j, raw, contiguousHeight, net) {
		return contiguousHeight + 1
	}
	return 0
}

// LowestMissingAfterContiguous returns the first height at or after contiguous+1 whose raw
// block is missing. This is O(1) and matches Core forward-IBD (fill from the chain tip frontier).
func LowestMissingAfterContiguous(j *HeaderJournal, raw *RawBlockStore, contiguous, tip int64, net chain.Network) (int64, error) {
	if j == nil || raw == nil {
		return -1, nil
	}
	start := int64(0)
	if contiguous >= 0 {
		if HasStoredBodyAtHeight(j, raw, contiguous, net) {
			start = contiguous + 1
		} else {
			start = contiguous
		}
	}
	if start > tip {
		return -1, nil
	}
	if !HasStoredBodyAtHeight(j, raw, start, net) {
		return start, nil
	}
	return -1, nil
}

// LowestMissingBlockHeightFrom finds the first missing body in [searchStart, tip] or, when
// searchStart > 0 and none is found, scans [0, searchStart-1] for an earlier gap.
func LowestMissingBlockHeightFrom(j *HeaderJournal, raw *RawBlockStore, searchStart, tip int64, net chain.Network) (int64, error) {
	if searchStart < 0 {
		searchStart = 0
	}
	low, err := LowestMissingBlockHeight(j, raw, searchStart, tip, net)
	if err != nil || low >= 0 || searchStart == 0 {
		return low, err
	}
	return LowestMissingBlockHeight(j, raw, 0, searchStart-1, net)
}
