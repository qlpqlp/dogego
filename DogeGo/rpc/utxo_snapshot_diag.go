// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"os"

	"dogego/store"
)

// mergeUtxoSnapshotDiagnostics adds operator-visible UTXO cache fields to getblockchaininfo.
func mergeUtxoSnapshotDiagnostics(res map[string]interface{}, paths *DataPaths, chainActive int64) {
	if res == nil {
		return
	}
	memTip := int64(-1)
	memOuts := 0
	if paths != nil && paths.Utxo != nil {
		memTip = paths.Utxo.TipHeight()
		memOuts = paths.Utxo.Count()
	}
	if memTip >= 0 {
		res["dogego_utxo_chain_active"] = memTip
		res["dogego_utxo_outputs"] = memOuts
	}
	if paths == nil || paths.ChainDataDir == "" {
		return
	}
	path := store.UtxoSnapshotPath(paths.ChainDataDir)
	res["dogego_utxo_snapshot_path"] = path
	diskTip, modUnix, err := store.ReadUtxoSnapshotDiskMeta(path)
	if err != nil {
		res["dogego_utxo_snapshot_error"] = err.Error()
		return
	}
	if diskTip < 0 {
		return
	}
	res["dogego_utxo_snapshot_height"] = diskTip
	if modUnix > 0 {
		res["dogego_utxo_snapshot_mtime"] = modUnix
	}
	if st, err := os.Stat(path); err == nil {
		res["dogego_utxo_snapshot_bytes"] = st.Size()
	}
	if memTip >= 0 && diskTip != memTip {
		res["dogego_utxo_snapshot_behind_chain_active"] = memTip - diskTip
	}
	if chainActive >= 0 && diskTip < chainActive {
		res["dogego_utxo_snapshot_lag_blocks"] = chainActive - diskTip
	}
	if store.UtxoSnapshotSaveInFlight() {
		res["dogego_utxo_snapshot_save_in_flight"] = true
	}
	if SyncUtxoRPCInFlight() {
		res["dogego_syncutxo_in_flight"] = true
	}
	if paths != nil && paths.UtxoConnectInFlight != nil && paths.UtxoConnectInFlight() {
		res["dogego_utxo_connect_in_flight"] = true
	}
	if paths != nil && paths.ContiguousRawHeight != nil {
		cont := paths.ContiguousRawHeight()
		if cont >= 0 {
			res["dogego_contiguous_raw_height"] = cont
			tip := memTip
			if tip < 0 {
				tip = diskTip
			}
			if tip >= 0 {
				aligned := tip <= cont+32
				if paths.UtxoBodiesAligned != nil {
					aligned = paths.UtxoBodiesAligned()
				}
				res["dogego_utxo_bodies_aligned"] = aligned
				if !aligned && tip > cont {
					res["dogego_utxo_body_replay_remaining"] = tip - cont
					if tip > 0 {
						res["dogego_snapshot_body_replay_pct"] = float64(cont+1) / float64(tip+1) * 100.0
					}
				}
				res["dogego_utxo_replay_target"] = tip
			}
		}
	}
}

// UtxoReplaySummaryKeys are operator-facing replay fields surfaced on /api/summary and the web UI.
var UtxoReplaySummaryKeys = []string{
	"dogego_utxo_bodies_aligned",
	"dogego_utxo_body_replay_remaining",
	"dogego_snapshot_body_replay_pct",
	"dogego_utxo_replay_target",
	"dogego_utxo_connect_in_flight",
	"dogego_utxo_snapshot_save_in_flight",
}

// MergeUtxoOperatorSummary adds UTXO cache / body-replay diagnostics (getblockchaininfo parity).
func MergeUtxoOperatorSummary(res map[string]interface{}, paths *DataPaths, chainActive int64) {
	mergeUtxoSnapshotDiagnostics(res, paths, chainActive)
}

// CopyUtxoReplaySummary copies replay operator fields from src into dst (e.g. capabilities live strip).
func CopyUtxoReplaySummary(dst, src map[string]interface{}) {
	if dst == nil || src == nil {
		return
	}
	for _, k := range UtxoReplaySummaryKeys {
		if v, ok := src[k]; ok {
			dst[k] = v
		}
	}
}
