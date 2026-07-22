// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/store"
)

func TestMergeUtxoSnapshotDiagnosticsReplayFields(t *testing.T) {
	dir := t.TempDir()
	u := store.NewUtxoCache()
	u.SetTipHeightForTest(5000)
	path := store.UtxoSnapshotPath(dir)
	if err := u.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	cont := int64(4489)
	res := map[string]interface{}{}
	mergeUtxoSnapshotDiagnostics(res, &DataPaths{
		ChainDataDir: dir,
		Utxo:         u,
		ContiguousRawHeight: func() int64 { return cont },
	}, 100)
	if res["dogego_utxo_bodies_aligned"] != false {
		t.Fatalf("aligned %v", res["dogego_utxo_bodies_aligned"])
	}
	if res["dogego_utxo_body_replay_remaining"] != int64(511) {
		t.Fatalf("remain %v", res["dogego_utxo_body_replay_remaining"])
	}
	dst := map[string]interface{}{}
	CopyUtxoReplaySummary(dst, res)
	if dst["dogego_utxo_body_replay_remaining"] != int64(511) {
		t.Fatalf("copy %v", dst)
	}
}

func TestMergeUtxoSnapshotDiagnostics(t *testing.T) {
	dir := t.TempDir()
	u := store.NewUtxoCache()
	u.SetTipHeightForTest(500)
	path := store.UtxoSnapshotPath(dir)
	if err := u.SaveSnapshot(path); err != nil {
		t.Fatal(err)
	}
	res := map[string]interface{}{}
	mergeUtxoSnapshotDiagnostics(res, &DataPaths{
		ChainDataDir: dir,
		Utxo:         u,
	}, 520)
	if res["dogego_utxo_chain_active"] != int64(500) {
		t.Fatalf("chain_active %v", res["dogego_utxo_chain_active"])
	}
	if res["dogego_utxo_snapshot_height"] != int64(500) {
		t.Fatalf("snapshot height %v", res["dogego_utxo_snapshot_height"])
	}
	if res["dogego_utxo_snapshot_lag_blocks"] != int64(20) {
		t.Fatalf("lag %v", res["dogego_utxo_snapshot_lag_blocks"])
	}
}
