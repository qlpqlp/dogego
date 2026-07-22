// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
)

// noopPebbleLogger silences Pebble WAL/compaction logs on stderr.
type noopPebbleLogger struct{}

func (noopPebbleLogger) Infof(string, ...interface{})  {}
func (noopPebbleLogger) Errorf(string, ...interface{}) {}
func (noopPebbleLogger) Fatalf(string, ...interface{}) {}

const schemaVersion = 2

// Key layout:
//
//	m/<key>           -> meta string
//	p/<subsystem>     -> index progress JSON
//	s/<seq:8 BE>      -> metric sample JSON
//	r/<seq:8 BE>      -> reorg event JSON
var (
	prefixMeta     = []byte("m/")
	prefixProgress = []byte("p/")
	prefixSample   = []byte("s/")
	keyMetaSchema  = []byte("m/schema_version")
	keyMetricSeq   = []byte("m/metric_seq")
)

// DB is the analytics Pebble store (indexer checkpoints + metric timeline).
type DB struct {
	db   *pebble.DB
	path string
}

func metaKey(k string) []byte {
	return append(append([]byte{}, prefixMeta...), k...)
}

func progressKey(subsystem string) []byte {
	return append(append([]byte{}, prefixProgress...), subsystem...)
}

func sampleKey(seq uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], seq)
	out := make([]byte, 0, len(prefixSample)+8)
	out = append(out, prefixSample...)
	return append(out, b[:]...)
}

func prefixEnd(p []byte) []byte {
	end := make([]byte, len(p))
	copy(end, p)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil
}

type indexProgressValue struct {
	LastHeight  int64 `json:"last_height"`
	UpdatedUnix int64 `json:"updated_unix"`
}

// Open opens (creating if needed) the analytics Pebble directory at dbPath.
// It is separate from DogeGo's headers/rawblocks store and from Core chainstate/.
func Open(dbPath string) (*DB, error) {
	return openDB(dbPath, false)
}

// OpenReadOnly opens an existing analytics Pebble directory for reads only.
// Use this when the embedded sidecar already holds a read-write handle (same process).
func OpenReadOnly(dbPath string) (*DB, error) {
	return openDB(dbPath, true)
}

func openDB(dbPath string, readOnly bool) (*DB, error) {
	dbPath = filepath.Clean(dbPath)
	if !readOnly {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			return nil, err
		}
	}
	opts := &pebble.Options{Logger: noopPebbleLogger{}, ReadOnly: readOnly}
	pdb, err := pebble.Open(dbPath, opts)
	if err != nil {
		if readOnly {
			return nil, fmt.Errorf("analytics open read-only: %w", err)
		}
		return nil, fmt.Errorf("analytics open: %w", err)
	}
	w := &DB{db: pdb, path: dbPath}
	if readOnly {
		return w, nil
	}
	if err := w.migrate(); err != nil {
		_ = pdb.Close()
		return nil, err
	}
	return w, nil
}

func (w *DB) migrate() error {
	_, closer, err := w.db.Get(keyMetaSchema)
	if err == pebble.ErrNotFound {
		if err := w.db.Set(keyMetaSchema, []byte(fmt.Sprintf("%d", schemaVersion)), pebble.Sync); err != nil {
			return err
		}
		return w.db.Set(metaKey("engine"), []byte("dogego-analytics-pebble"), pebble.Sync)
	}
	if err != nil {
		return fmt.Errorf("analytics migrate: %w", err)
	}
	_ = closer.Close()
	// Additive: reorg event prefix (r/) needs no rewrite; bump stored schema when below current.
	if SchemaVersion(w) < schemaVersion {
		if err := w.db.Set(keyMetaSchema, []byte(fmt.Sprintf("%d", schemaVersion)), pebble.Sync); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the analytics store.
func (w *DB) Close() error {
	if w == nil || w.db == nil {
		return nil
	}
	db := w.db
	w.db = nil
	return db.Close()
}

// SetMeta upserts a key in dogego_meta.
func SetMeta(db *DB, k, v string) error {
	if db == nil || db.db == nil {
		return fmt.Errorf("analytics db closed")
	}
	return db.db.Set(metaKey(k), []byte(v), pebble.Sync)
}

// SchemaVersion returns the stored schema version or 0 if unreadable.
func SchemaVersion(db *DB) int {
	if db == nil || db.db == nil {
		return 0
	}
	val, closer, err := db.db.Get(keyMetaSchema)
	if err != nil {
		return 0
	}
	defer closer.Close()
	var n int
	_, _ = fmt.Sscanf(strings.TrimSpace(string(val)), "%d", &n)
	return n
}

func setIndexProgress(db *DB, subsystem string, lastHeight, updatedUnix int64) error {
	if db == nil || db.db == nil {
		return fmt.Errorf("analytics db closed")
	}
	b, err := json.Marshal(indexProgressValue{LastHeight: lastHeight, UpdatedUnix: updatedUnix})
	if err != nil {
		return err
	}
	return db.db.Set(progressKey(subsystem), b, pebble.Sync)
}

// RecordRawBlockScan stores how many *.bin files exist under rawblocks/.
// index_progress.last_height holds the count for subsystem 'rawblocks' (not a chain height).
func RecordRawBlockScan(db *DB, binCount int) error {
	return setIndexProgress(db, "rawblocks", int64(binCount), time.Now().Unix())
}

// RecordHeadersSynced stores the local header journal tip height in index_progress subsystem 'headers'.
func RecordHeadersSynced(db *DB, tipHeight int64) error {
	return setIndexProgress(db, "headers", tipHeight, time.Now().Unix())
}

func listIndexProgress(db *DB) ([]IndexProgressRow, error) {
	it, err := db.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixProgress,
		UpperBound: prefixEnd(prefixProgress),
	})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var out []IndexProgressRow
	for ok := it.First(); ok; ok = it.Next() {
		sub := string(it.Key()[len(prefixProgress):])
		var v indexProgressValue
		if err := json.Unmarshal(it.Value(), &v); err != nil {
			return nil, err
		}
		out = append(out, IndexProgressRow{
			Subsystem: sub, LastHeight: v.LastHeight, UpdatedUnix: v.UpdatedUnix,
		})
	}
	return out, it.Error()
}

func listMeta(db *DB) (map[string]string, error) {
	it, err := db.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixMeta,
		UpperBound: prefixEnd(prefixMeta),
	})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	out := make(map[string]string)
	for ok := it.First(); ok; ok = it.Next() {
		k := string(it.Key()[len(prefixMeta):])
		out[k] = string(it.Value())
	}
	return out, it.Error()
}

func nextMetricSeq(db *DB) (uint64, error) {
	val, closer, err := db.db.Get(keyMetricSeq)
	seq := uint64(0)
	if err == nil {
		defer closer.Close()
		_, _ = fmt.Sscanf(strings.TrimSpace(string(val)), "%d", &seq)
	}
	seq++
	if err := db.db.Set(keyMetricSeq, []byte(fmt.Sprintf("%d", seq)), pebble.Sync); err != nil {
		return 0, err
	}
	return seq, nil
}

func pruneMetricSamples(db *DB, keepFrom uint64) error {
	if keepFrom <= 1 {
		return nil
	}
	return db.db.DeleteRange(sampleKey(1), sampleKey(keepFrom), pebble.Sync)
}

// analyticsStoreExists reports whether dbPath is an existing analytics Pebble directory.
func analyticsStoreExists(dbPath string) bool {
	st, err := os.Stat(dbPath)
	return err == nil && st.IsDir()
}
