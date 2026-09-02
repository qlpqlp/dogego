// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "dogego/chain"

// ContiguousTipSpotOK reports whether height looks like a valid contiguous tip on disk
// (body at height and height-1). Used to skip full startup rescan after a clean exit.
func ContiguousTipSpotOK(j *HeaderJournal, raw *RawBlockStore, net chain.Network, height int64) bool {
	return contiguousSpotTip(j, raw, net, height) >= 0
}

// MeasureContiguousBodiesOnDisk returns the highest height h where every height in [0,h]
// has an adequate stored body. Seeds from seed when that height is contiguous on disk;
// scans forward at most limit steps (0 = unlimited).
func MeasureContiguousBodiesOnDisk(j *HeaderJournal, raw *RawBlockStore, net chain.Network, seed, limit int64) int64 {
	return MeasureContiguousBodiesOnDiskTarget(j, raw, net, seed, limit, -1)
}

// MeasureContiguousBodiesOnDiskTarget is MeasureContiguousBodiesOnDisk with an optional
// expected tip for WebUI progress (target < 0 when unknown).
func MeasureContiguousBodiesOnDiskTarget(j *HeaderJournal, raw *RawBlockStore, net chain.Network, seed, limit, target int64) int64 {
	if j == nil || raw == nil {
		return -1
	}
	start := resolveContiguousMeasureStart(j, raw, net, seed, target)
	if start < 0 {
		return -1
	}
	h := start
	steps := int64(0)
	end := target
	if end < start {
		end = start
	}
	reportContiguousMeasure(h, start, end)
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
		if steps == 1 || steps%4096 == 0 {
			reportContiguousMeasure(h, start, end)
		}
	}
	reportContiguousMeasure(h, start, end)
	return h
}

// resolveContiguousMeasureStart picks a measure origin near seed/target instead of always
// falling back to genesis (which can take minutes and freezes P2P startup).
func resolveContiguousMeasureStart(j *HeaderJournal, raw *RawBlockStore, net chain.Network, seed, target int64) int64 {
	if tip := contiguousSpotTip(j, raw, net, seed); tip >= 0 {
		return tip
	}
	if tip := contiguousSpotTip(j, raw, net, target); tip >= 0 {
		return tip
	}
	from := seed
	if target > from {
		from = target
	}
	if from >= 0 {
		if tip := walkBackContiguousSpot(j, raw, net, from, 65536); tip >= 0 {
			return tip
		}
		// Prefer staying near the checkpoint over a multi-minute genesis walk.
		if seed >= 0 {
			return seed
		}
		if target >= 0 {
			return target
		}
	}
	if HasStoredBodyAtHeight(j, raw, 0, net) {
		return 0
	}
	return -1
}

func contiguousSpotTip(j *HeaderJournal, raw *RawBlockStore, net chain.Network, h int64) int64 {
	if h < 0 || !HasStoredBodyAtHeight(j, raw, h, net) {
		return -1
	}
	if h > 0 && !HasStoredBodyAtHeight(j, raw, h-1, net) {
		return -1
	}
	return h
}

func walkBackContiguousSpot(j *HeaderJournal, raw *RawBlockStore, net chain.Network, from, maxSteps int64) int64 {
	if from < 0 {
		return -1
	}
	if maxSteps < 0 {
		maxSteps = 0
	}
	for i := int64(0); i <= maxSteps; i++ {
		h := from - i
		if h < 0 {
			break
		}
		if tip := contiguousSpotTip(j, raw, net, h); tip >= 0 {
			return tip
		}
	}
	return -1
}

// ReconcileBundledContiguousTip returns readable body coverage for bundled (or hybrid) stores.
func ReconcileBundledContiguousTip(j *HeaderJournal, raw *RawBlockStore, net chain.Network) int64 {
	return ReconcileBundledContiguousTipSeeded(j, raw, net, -1)
}

// ReconcileBundledContiguousTipSeeded is ReconcileBundledContiguousTip with an optional prior
// contiguous tip (e.g. rawblocks_sync.json). When seed is valid on disk, journal measurement
// starts there instead of height 0 so restarts stay O(delta) rather than O(height).
//
// ProbeBundledContiguousTip only scans blk*.dat. After a perfile→bundled upgrade, leftover
// *.bin bodies remain readable via HasStoredBody; preferring probe alone falsely rewinds
// contiguous coverage (e.g. ~200k → a few thousand). When legacy *.bin exist, journal
// measurement wins. Pure bundled stores keep the conservative min(probe, measured) behavior
// so torn blk tails still clamp.
func ReconcileBundledContiguousTipSeeded(j *HeaderJournal, raw *RawBlockStore, net chain.Network, seed int64) int64 {
	if j == nil || raw == nil {
		return -1
	}
	BeginContiguousReconcile()
	defer EndContiguousReconcile()
	// Clean checkpoint tip: skip the multi-minute blk*.dat stream probe. Walk forward
	// from the seed only so new bodies since last exit are still discovered (O(delta)).
	if seed >= 0 && ContiguousTipSpotOK(j, raw, net, seed) {
		reportContiguousProbeDone()
		grown := MeasureContiguousBodiesOnDiskTarget(j, raw, net, seed, 0, -1)
		if grown >= seed {
			reportContiguousCheckpointVerified(grown)
			return grown
		}
		reportContiguousCheckpointVerified(seed)
		return seed
	}
	probe, err := raw.ProbeBundledContiguousTip()
	if err != nil {
		probe = -1
	}
	measureSeed := seed
	if measureSeed < 0 {
		measureSeed = 0
	}
	expected := probe
	if seed > expected {
		expected = seed
	}

	if raw.HasLegacyPerFileBodies() {
		legacySeed := probe
		if legacySeed < 0 {
			legacySeed = measureSeed
		}
		return MeasureContiguousBodiesOnDiskTarget(j, raw, net, legacySeed, 0, expected)
	}

	// Pure bundled: append-order probe is the coverage ceiling. Prefer spotting the tip
	// (or walking back a bounded window) over a multi-minute genesis rescan.
	if probe >= 0 {
		if tip := contiguousSpotTip(j, raw, net, probe); tip >= 0 {
			if seed >= 0 && seed < tip {
				measured := MeasureContiguousBodiesOnDiskTarget(j, raw, net, seed, 0, tip)
				if measured >= 0 && measured < tip {
					return measured
				}
			}
			return tip
		}
		if tip := walkBackContiguousSpot(j, raw, net, probe, 65536); tip >= 0 {
			// Tip locators near probe may be missing; do not re-measure toward probe from a
			// distant walk-back origin (that becomes another long scan). Trust walk-back tip.
			return tip
		}
		if seed >= 0 {
			return seed
		}
		return probe
	}

	if seed >= 0 {
		if tip := contiguousSpotTip(j, raw, net, seed); tip >= 0 {
			return tip
		}
		if tip := walkBackContiguousSpot(j, raw, net, seed, 65536); tip >= 0 {
			return tip
		}
		return seed
	}

	return MeasureContiguousBodiesOnDiskTarget(j, raw, net, 0, 0, expected)
}

// LightVerifyBundledContiguousTip is the cheap startup check: spot-check the checkpoint tip.
// When the tip is still present on disk it skips the multi-minute full blk*.dat stream probe
// (IBD already resumed from rawblocks_sync.json). It never walks the journal from genesis.
//
// Full ProbeBundledContiguousTip only runs when there is no trusted seed tip (unclean exit /
// missing bodies) so operators do not see "Scanning blk00000.dat…" on every clean restart.
func LightVerifyBundledContiguousTip(j *HeaderJournal, raw *RawBlockStore, net chain.Network, seed int64) int64 {
	if j == nil || raw == nil {
		return -1
	}
	BeginContiguousReconcile()
	defer EndContiguousReconcile()
	seedOK := ContiguousTipSpotOK(j, raw, net, seed)
	if seed >= 0 && seedOK {
		reportContiguousProbeDone()
		// Spot-check only: walk forward from seed for any bodies written since last exit.
		grown := MeasureContiguousBodiesOnDiskTarget(j, raw, net, seed, 0, -1)
		if grown >= seed {
			reportContiguousCheckpointVerified(grown)
			return grown
		}
		reportContiguousCheckpointVerified(seed)
		return seed
	}
	// No trusted checkpoint tip — fall back to a full append-order probe.
	probe, err := raw.ProbeBundledContiguousTip()
	if err != nil {
		probe = -1
	}
	if probe >= 0 {
		reportContiguousProbeDone()
		if tip := contiguousSpotTip(j, raw, net, probe); tip >= 0 {
			reportContiguousMeasure(tip, tip, tip)
			return tip
		}
		if tip := walkBackContiguousSpot(j, raw, net, probe, 4096); tip >= 0 {
			reportContiguousMeasure(tip, tip, probe)
			return tip
		}
		return probe
	}
	if seed >= 0 {
		if tip := contiguousSpotTip(j, raw, net, seed); tip >= 0 {
			reportContiguousMeasure(tip, tip, tip)
			return tip
		}
		return seed
	}
	return -1
}
