// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "dogego/chain"

// MeasureContiguousBodiesOnDisk returns the highest height h where every height in [0,h]
// has an adequate stored body. Seeds from seed when that height is contiguous on disk;
// scans forward at most limit steps (0 = unlimited).
func MeasureContiguousBodiesOnDisk(j *HeaderJournal, raw *RawBlockStore, net chain.Network, seed, limit int64) int64 {
	if j == nil || raw == nil {
		return -1
	}
	start := int64(-1)
	if seed >= 0 && HasStoredBodyAtHeight(j, raw, seed, net) {
		if seed == 0 || HasStoredBodyAtHeight(j, raw, seed-1, net) {
			start = seed
		}
	}
	if start < 0 {
		if !HasStoredBodyAtHeight(j, raw, 0, net) {
			return -1
		}
		start = 0
	}
	h := start
	steps := int64(0)
	for {
		if limit > 0 && steps >= limit {
			break
		}
		next := h + 1
		if _, err := j.ReadHeaderAt(next); err != nil {
			break
		}
		if !HasStoredBodyAtHeight(j, raw, next, net) {
			break
		}
		h = next
		steps++
	}
	return h
}

// ReconcileBundledContiguousTip returns readable body coverage for bundled (or hybrid) stores.
//
// ProbeBundledContiguousTip only scans blk*.dat. After a perfile→bundled upgrade, leftover
// *.bin bodies remain readable via HasStoredBody; preferring probe alone falsely rewinds
// contiguous coverage (e.g. ~200k → a few thousand). When legacy *.bin exist, journal
// measurement wins. Pure bundled stores keep the conservative min(probe, measured) behavior
// so torn blk tails still clamp.
func ReconcileBundledContiguousTip(j *HeaderJournal, raw *RawBlockStore, net chain.Network) int64 {
	if j == nil || raw == nil {
		return -1
	}
	probe, err := raw.ProbeBundledContiguousTip()
	if err != nil {
		probe = -1
	}
	measured := MeasureContiguousBodiesOnDisk(j, raw, net, 0, 0)
	switch {
	case probe < 0 && measured < 0:
		return -1
	case measured < 0:
		return probe
	case probe < 0:
		return measured
	case raw.HasLegacyPerFileBodies():
		// Extend from bundled tip through leftover *.bin (avoid full genesis rescan).
		return MeasureContiguousBodiesOnDisk(j, raw, net, probe, 0)
	case measured < probe:
		return measured
	default:
		return probe
	}
}
