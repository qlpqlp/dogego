// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestCoreFeeEstimatesDatRoundtrip(t *testing.T) {
	stats := NewTxConfirmStats()
	stats.RecordConfirm(2, 200_000)
	stats.FlushBlock()
	stats.SetBestSeenHeight(42)

	dir := t.TempDir()
	path := filepath.Join(dir, "fee_estimates.dat")
	if err := WriteCoreFeeEstimatesDat(path, 42, stats); err != nil {
		t.Fatal(err)
	}
	best, loaded, err := ReadCoreFeeEstimatesDat(path)
	if err != nil {
		t.Fatal(err)
	}
	if best != 42 {
		t.Fatalf("best seen %d want 42", best)
	}
	if loaded == nil {
		t.Fatal("nil stats")
	}
	if len(loaded.buckets) != len(stats.buckets) {
		t.Fatalf("buckets %d want %d", len(loaded.buckets), len(stats.buckets))
	}
	if loaded.decay != stats.decay {
		t.Fatalf("decay %v want %v", loaded.decay, stats.decay)
	}
	if loaded.bestSeenHeight != 42 {
		t.Fatalf("height %d want 42", loaded.bestSeenHeight)
	}
}

func TestFeeHistorySaveLoadCoreDat(t *testing.T) {
	h := NewFeeHistory(8)
	h.confirmStats = NewTxConfirmStats()
	h.confirmStats.RecordConfirm(1, 150_000)
	h.confirmStats.FlushBlock()
	h.confirmStats.SetBestSeenHeight(10)

	dir := t.TempDir()
	dat := filepath.Join(dir, "fee_estimates.dat")
	if err := h.SaveCoreFeeEstimatesDat(dat); err != nil {
		t.Fatal(err)
	}
	best, stats, err := ReadCoreFeeEstimatesDat(dat)
	if err != nil {
		t.Fatal(err)
	}
	h2 := NewFeeHistory(8)
	h2.ApplyCoreConfirmStats(best, stats)
	if h2.confirmStats == nil {
		t.Fatal("no stats after apply")
	}
	if h2.confirmStats.bestSeenHeight != 10 {
		t.Fatalf("height %d", h2.confirmStats.bestSeenHeight)
	}
}

func TestReadCoreFeeEstimatesDatMissing(t *testing.T) {
	_, stats, err := ReadCoreFeeEstimatesDat(filepath.Join(t.TempDir(), "missing.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if stats != nil {
		t.Fatal("expected nil stats")
	}
}

func TestReadCoreFeeEstimatesDatSkipsLegacyPriorityStats(t *testing.T) {
	stats := NewTxConfirmStats()
	stats.RecordConfirm(1, 100_000)
	stats.FlushBlock()
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, int32(7)); err != nil {
		t.Fatal(err)
	}
	if err := writeCoreTxConfirmStats(&buf, stats); err != nil {
		t.Fatal(err)
	}
	pri := newTxConfirmStats(DefaultFeerateBucketUpperBounds(), 5, defaultConfirmStatsDecay)
	if err := writeCoreTxConfirmStats(&buf, pri); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.dat")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	best, loaded, err := ReadCoreFeeEstimatesDat(path)
	if err != nil {
		t.Fatal(err)
	}
	if best != 7 || loaded == nil {
		t.Fatalf("best=%d stats=%v", best, loaded)
	}
}

func TestReadCoreFeeEstimatesDatCorruptDecay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.dat")
	if err := os.WriteFile(path, []byte{0, 0, 0, 0}, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := ReadCoreFeeEstimatesDat(path)
	if err == nil {
		t.Fatal("want error")
	}
}
