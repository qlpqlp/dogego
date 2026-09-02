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
	contigBPM, _ := snap["contiguous_blocks_per_minute"].(float64)
	inflight := snapIntFlexible(snap["in_flight_batches"])
	probe := snapInt64Flexible(snap["next_probe_height"])
	workers := snapIntFlexible(snap["sync_workers"])
	low := snapInt64Flexible(snap["lowest_missing_height"])
	cont := snapInt64Flexible(snap["contiguous_raw_height"])
	if low < 0 {
		// enrich may omit the field; fall back to contiguous frontier for the log line.
		if cont >= 0 {
			low = cont + 1
		} else {
			low = 0
		}
	}
	applog.Line("block", fmt.Sprintf("IBD: %d blocks stored this run, %.1f ingest blk/min, %.1f stored blk/min, lowest missing %d, bodies through %d, probe %d, %d in flight, %d lane(s)",
		stored, bpm, contigBPM, low, cont, probe, inflight, workers))
}

func snapIntFlexible(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

func snapInt64Flexible(v interface{}) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	default:
		return -1
	}
}
