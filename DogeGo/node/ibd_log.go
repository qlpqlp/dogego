// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/applog"
	"dogego/store"
)

// MaybeLogIBDProgress logs block download rate periodically during forward IBD.
func (s *progressiveRawState) MaybeLogIBDProgress(j *store.HeaderJournal, bs *BlockStoreCtx) {
	if s == nil {
		return
	}
	s.mu.Lock()
	idle := s.idleFull
	stored := s.blocksStoredIBD
	s.mu.Unlock()
	if idle {
		return
	}
	snap := s.snapshot()
	enrichIBDProgressSnapshot(snap, j, bs)
	bpm, _ := snap["blocks_per_minute"].(float64)
	inflight, _ := snap["in_flight_batches"].(int)
	probe, _ := snap["next_probe_height"].(int64)
	workers, _ := snap["sync_workers"].(int)
	low, _ := snap["lowest_missing_height"].(int64)
	cont, _ := snap["contiguous_raw_height"].(int64)
	applog.Line("block", fmt.Sprintf("IBD: %d blocks stored this run, %.1f blk/min, lowest missing %d, bodies through %d, probe %d, %d in flight, %d lane(s)",
		stored, bpm, low, cont, probe, inflight, workers))
}
