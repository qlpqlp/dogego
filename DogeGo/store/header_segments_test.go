// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dogego/pow"
)

func TestHeaderSegmentReadCache(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	batch := make([]byte, 80*3)
	for i := 0; i < 3; i++ {
		copy(batch[i*80:(i+1)*80], gen)
		binaryPutNonce(batch[i*80:], uint32(i+1))
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	h1, err := j.ReadHeaderAt(2)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := j.ReadHeaderAt(2)
	if err != nil {
		t.Fatal(err)
	}
	if string(h1) != string(h2) {
		t.Fatal("cached segment read mismatch")
	}
}

func TestHeaderSegmentAppendTruncate(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	if j.HeaderLayout() != headerLayoutSegments {
		t.Fatalf("layout %q want segments", j.HeaderLayout())
	}
	batch := make([]byte, 80*5)
	for i := 0; i < 5; i++ {
		copy(batch[i*80:(i+1)*80], gen)
		binaryPutNonce(batch[i*80:], uint32(i+1))
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	tip, err := j.TipHeight()
	if err != nil || tip != 5 {
		t.Fatalf("tip=%d err=%v want 5", tip, err)
	}
	if err := j.TruncateToHeight(2); err != nil {
		t.Fatal(err)
	}
	tip, _ = j.TipHeight()
	if tip != 2 {
		t.Fatalf("after truncate tip=%d want 2", tip)
	}
	best, err := j.BestBlockHashHex()
	if err != nil || best == "" {
		t.Fatalf("BestBlockHashHex: %q err=%v", best, err)
	}
	cp, err := LoadHeaderSyncCheckpoint(dir)
	if err != nil || cp.TipHeight != 2 {
		t.Fatalf("checkpoint tip=%d err=%v", cp.TipHeight, err)
	}
}

func TestMigrateMonolithToSegments(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	mono := dir + "/headers.bin"
	j, err := OpenHeaderJournal(mono, gen)
	if err != nil {
		t.Fatal(err)
	}
	batch := make([]byte, 80*3)
	for i := 0; i < 3; i++ {
		copy(batch[i*80:(i+1)*80], gen)
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	l, err := migrateMonolithToSegments(dir, mono)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mono); !os.IsNotExist(err) {
		t.Fatal("expected headers.bin renamed")
	}
	if l.recordCount() != 4 {
		t.Fatalf("count=%d want 4", l.recordCount())
	}
}

func TestHeaderSegmentTailRepairOnOpen(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	batch := make([]byte, 80*3)
	for i := 0; i < 3; i++ {
		copy(batch[i*80:(i+1)*80], gen)
		binaryPutNonce(batch[i*80:], uint32(i+1))
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	seg := filepath.Join(dir, "headers", "seg", "0000000000.bin")
	f, err := os.OpenFile(seg, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 41)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()

	j2, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	tip, _ := j2.TipHeight()
	if tip != 3 {
		t.Fatalf("repaired tip=%d want 3", tip)
	}
}

func TestHeaderSegmentCheckpointRepair(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	batch := make([]byte, 80*5)
	for i := 0; i < 5; i++ {
		copy(batch[i*80:(i+1)*80], gen)
		binaryPutNonce(batch[i*80:], uint32(i+1))
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	if err := SaveHeaderSyncCheckpoint(dir, HeaderSyncCheckpoint{
		Layout: headerLayoutSegments, TipHeight: 2, HeaderCount: 3,
	}); err != nil {
		t.Fatal(err)
	}
	// Simulate crash: manifest/checkpoint stale but segment files grew (append without checkpoint).
	j2, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	tip, _ := j2.TipHeight()
	if tip != 2 {
		t.Fatalf("checkpoint repair tip=%d want 2", tip)
	}
}

func TestHeaderSegmentPurgeStaleTemps(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	segDir := filepath.Join(dir, "headers", "seg")
	if err := os.WriteFile(filepath.Join(segDir, "0000000000.bin.tmp"), gen, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "0000002000.bin.tmp"), gen, 0o600); err != nil {
		t.Fatal(err)
	}
	j2, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	_ = j2
	if _, err := os.Stat(filepath.Join(segDir, "0000000000.bin.tmp")); !os.IsNotExist(err) {
		t.Fatalf("stale tmp 0: %v", err)
	}
	if _, err := os.Stat(filepath.Join(segDir, "0000002000.bin.tmp")); !os.IsNotExist(err) {
		t.Fatalf("stale tmp 2000: %v", err)
	}
	_ = j
}

func TestHeaderSegmentAppendRejectsInvalidBatchSize(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.AppendWireHeaderBatch(make([]byte, 79)); err == nil {
		t.Fatal("expected append to reject non-80-byte-aligned batch")
	}
	tip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	if tip != 0 {
		t.Fatalf("tip=%d want 0 (genesis only)", tip)
	}
	cp, err := LoadHeaderSyncCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cp.TipHeight != 0 || cp.HeaderCount != 1 {
		t.Fatalf("checkpoint after rejected append tip=%d count=%d want 0/1", cp.TipHeight, cp.HeaderCount)
	}
}

func TestHeaderSegmentAppendRejectsEmptyBatch(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.AppendWireHeaderBatch(nil); err == nil {
		t.Fatal("expected append to reject empty batch")
	}
	tip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	if tip != 0 {
		t.Fatalf("tip=%d want 0 after rejected empty append", tip)
	}
}

func TestHeaderSegmentAppendBatchInitializesGenesisAndRemainder(t *testing.T) {
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	// Create one batch that includes genesis + 2 following headers.
	batch := make([]byte, 80*3)
	for i := 0; i < 3; i++ {
		copy(batch[i*80:(i+1)*80], gen)
		binaryPutNonce(batch[i*80:], uint32(i+1))
	}
	if err := j.TruncateToHeight(0); err != nil {
		t.Fatal(err)
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	tip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	if tip != 3 {
		t.Fatalf("tip=%d want 3 (genesis + 3 appended headers)", tip)
	}
	h3, err := j.ReadHeaderAt(3)
	if err != nil {
		t.Fatal(err)
	}
	if h3[76] != 3 {
		t.Fatalf("height 3 nonce=%d want 3", h3[76])
	}
	cp, err := LoadHeaderSyncCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cp.TipHeight != 3 || cp.HeaderCount != 4 {
		t.Fatalf("checkpoint tip=%d count=%d want 3/4", cp.TipHeight, cp.HeaderCount)
	}
}

func binaryPutNonce(h []byte, n uint32) {
	if len(h) >= 80 {
		h[76] = byte(n)
		h[77] = byte(n >> 8)
		h[78] = byte(n >> 16)
		h[79] = byte(n >> 24)
	}
}

// TestHeaderSegmentAppendCrossesSegmentFile verifies appendBatch splits across seg/NNNNNNNNNN.bin files.
func TestHeaderSegmentAppendCrossesSegmentFile(t *testing.T) {
	const smallSeg = 4
	dir := t.TempDir()
	gen := make([]byte, 80)
	j, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	if j.HeaderLayout() != headerLayoutSegments {
		t.Fatalf("layout %q", j.HeaderLayout())
	}
	// Shrink segment size so the test crosses a boundary without 2000 headers.
	manifest := headerManifest{Version: 1, SegmentSize: smallSeg, TipHeight: 0, TipHashHex: pow.BlockHashHex(gen)}
	b, _ := json.Marshal(manifest)
	if err := os.WriteFile(headerManifestPath(dir), b, 0o600); err != nil {
		t.Fatal(err)
	}
	j2, err := OpenHeaderChain(dir, gen)
	if err != nil {
		t.Fatal(err)
	}
	j = j2
	nAppend := 6
	batch := make([]byte, 80*nAppend)
	for i := 0; i < nAppend; i++ {
		copy(batch[i*80:(i+1)*80], gen)
		binaryPutNonce(batch[i*80:], uint32(i+1))
	}
	if err := j.AppendWireHeaderBatch(batch); err != nil {
		t.Fatal(err)
	}
	tip, err := j.TipHeight()
	if err != nil || tip != int64(nAppend) {
		t.Fatalf("tip=%d err=%v want %d", tip, err, nAppend)
	}
	seg0 := filepath.Join(dir, headerLayoutDir, headerSegmentSubdir, "0000000000.bin")
	seg4 := filepath.Join(dir, headerLayoutDir, headerSegmentSubdir, "0000000004.bin")
	if _, err := os.Stat(seg0); err != nil {
		t.Fatalf("segment 0: %v", err)
	}
	if _, err := os.Stat(seg4); err != nil {
		t.Fatalf("segment 4: %v", err)
	}
	h5, err := j.ReadHeaderAt(5)
	if err != nil {
		t.Fatal(err)
	}
	if h5[76] != 5 {
		t.Fatalf("height 5 nonce=%d want 5", h5[76])
	}
}
