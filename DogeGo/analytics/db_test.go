// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "a.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if SchemaVersion(db) != schemaVersion {
		t.Fatalf("schema version %d", SchemaVersion(db))
	}
	if err := SetMeta(db, "smoke", "ok"); err != nil {
		t.Fatal(err)
	}
}

func TestRecordRawBlockScan(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "x.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordRawBlockScan(db, 42); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	ex, _, _, raw, err := ReadSideSummary(dbPath)
	if err != nil || !ex || raw == nil || *raw != 42 {
		t.Fatalf("summary ex=%v raw=%v err=%v", ex, raw, err)
	}
}

func TestRecordHeadersSynced(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "h.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordHeadersSynced(db, 99); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	d, err := ReadSideDetail(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, r := range d.IndexProgress {
		if r.Subsystem == "headers" && r.LastHeight == 99 {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("index_progress %#v", d.IndexProgress)
	}
}

func TestReadSideDetailMetaAndProgress(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "d.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetMeta(db, "chain_root", "/tmp/chain"); err != nil {
		t.Fatal(err)
	}
	if err := RecordRawBlockScan(db, 7); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	d, err := ReadSideDetail(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if d == nil || !d.Exists || d.Schema != schemaVersion {
		t.Fatalf("detail %+v", d)
	}
	if d.RawBinCount == nil || *d.RawBinCount != 7 {
		t.Fatalf("raw bin %v", d.RawBinCount)
	}
	if d.Meta["chain_root"] != "/tmp/chain" || d.Meta["schema_version"] == "" {
		t.Fatalf("meta %#v", d.Meta)
	}
	if len(d.IndexProgress) < 1 {
		t.Fatalf("index_progress %#v", d.IndexProgress)
	}
	var sawRaw bool
	for _, r := range d.IndexProgress {
		if r.Subsystem == "rawblocks" && r.LastHeight == 7 {
			sawRaw = true
		}
	}
	if !sawRaw {
		t.Fatalf("rows %#v", d.IndexProgress)
	}
}

func TestReadSideDetailWhileWriteOpen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "locked.db")
	shared, err := OpenShared(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shared.Close() })
	w := shared.Writer()
	if err := RecordHeadersSynced(w, 12); err != nil {
		t.Fatal(err)
	}
	if err := RecordMetricSample(w, LiveMetrics{MempoolTxs: 3, ChainDataBytes: 1024}); err != nil {
		t.Fatal(err)
	}

	d, err := shared.ReadDetail()
	if err != nil {
		t.Fatalf("ReadDetail while sidecar write handle open: %v", err)
	}
	if d == nil || !d.Exists || len(d.IndexProgress) == 0 || len(d.MetricTimeline) == 0 {
		t.Fatalf("detail %+v", d)
	}
}
