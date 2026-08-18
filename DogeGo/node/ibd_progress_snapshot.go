// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/store"
)

// enrichIBDProgressSnapshot adds live block-sync fields for dashboard / P2P ibd_progress.
func enrichIBDProgressSnapshot(snap map[string]interface{}, j *store.HeaderJournal, bs *BlockStoreCtx) {
	if snap == nil {
		return
	}
	if bs != nil {
		snap["genesis_missing"] = NeedsGenesisBlock(bs)
		cont := bs.ContiguousRawHeight()
		snap["contiguous_raw_height"] = cont
		snap["bodies_behind_headers"] = BodiesBehindHeaders(bs)
		if bs.DbCacheMB > 0 {
			snap["dbcache_mb"] = bs.DbCacheMB
		}
		if ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
			snap["body_ibd_header_paused"] = true
		}
		if ShouldDeferConnectForBodyDownload(bs) {
			snap["connect_deferred_for_download"] = true
		}
		snap["connect_catch_up_min_lag"] = EffectiveConnectCatchUpMinLag(bs)
		if bs.utxoAheadOfStoredBodies() && bs.Utxo != nil && bs.Utxo.TipHeight() >= 0 {
			snap["snapshot_body_replay"] = true
			snap["snapshot_body_replay_target"] = bs.Utxo.TipHeight()
			if cont >= 0 {
				snap["snapshot_body_replay_remaining"] = bs.Utxo.TipHeight() - cont
			}
		}
		if bs.Utxo != nil {
			if lag := ConnectCatchUpLag(bs, bs.Utxo); lag > 0 {
				snap["connect_lag"] = lag
				snap["connect_catch_up_passes"] = connectCatchUpPasses(lag, bs)
				snap["connect_catch_up_batch"] = connectCatchUpBlocksPerIBDCall(bs)
				snap["connect_catch_up_interval_ms"] = connectCatchUpInterval(lag).Milliseconds()
			}
			if gap := ConnectBodyGapHeight(bs); gap >= 0 {
				snap["connect_body_gap_height"] = gap
			}
			if rate := IBDConnectBlocksPerMinute(); rate > 0 {
				snap["connect_blocks_per_minute"] = rate
			}
		}
	}
	if bs == nil || j == nil || bs.Raw == nil {
		return
	}
	tip, _, err := j.SyncTipFromDisk()
	if err != nil || tip < 0 {
		return
	}
	cont := bs.ContiguousRawHeight()
	connectLag := int64(0)
	if bs.Utxo != nil {
		connectLag = ConnectCatchUpLag(bs, bs.Utxo)
	}
	// During deep connect catch-up the forward frontier is contiguous+1; avoid O(tip) gap scans on every RPC poll.
	if connectLag > 2048 {
		if low, err := store.LowestMissingAfterContiguous(j, bs.Raw, cont, tip, bs.chainNet()); err == nil && low >= 0 {
			snap["lowest_missing_height"] = low
		}
	} else {
		searchStart := store.LowestMissingSearchStart(j, bs.Raw, cont, bs.chainNet())
		if low, err := store.LowestMissingBlockHeightFrom(j, bs.Raw, searchStart, tip, bs.chainNet()); err == nil && low >= 0 {
			snap["lowest_missing_height"] = low
			if ahead := rawBlocksAheadOfContiguous(cont, low); ahead > 0 {
				snap["raw_blocks_ahead_of_contiguous"] = ahead
			}
		}
	}
	if cont >= 0 && tip > cont {
		snap["blocks_behind"] = tip - cont
	}
	bodyIBDEtaMinutes(snap, tip, cont)
}

func bodyIBDEtaMinutes(snap map[string]interface{}, tip, cont int64) {
	if snap == nil || tip <= cont || cont < 0 {
		return
	}
	bpm := float64(0)
	switch v := snap["blocks_per_minute"].(type) {
	case float64:
		bpm = v
	case int:
		bpm = float64(v)
	case int64:
		bpm = float64(v)
	}
	if bpm <= 0 {
		return
	}
	snap["body_ibd_eta_minutes"] = int64(float64(tip-cont)/bpm + 0.5)
}

// rawBlocksAheadOfContiguous counts stored-but-non-contiguous heights below lowest_missing.
func rawBlocksAheadOfContiguous(contiguous, lowestMissing int64) int64 {
	if contiguous < 0 || lowestMissing < 0 || lowestMissing <= contiguous+1 {
		return 0
	}
	return lowestMissing - contiguous - 1
}
