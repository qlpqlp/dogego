// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"sort"
)

// IndexProgressRow is one row from index_progress (last_height meaning depends on subsystem;
// for subsystem "rawblocks" it is the *.bin count under rawblocks/).
type IndexProgressRow struct {
	Subsystem   string `json:"subsystem"`
	LastHeight  int64  `json:"last_height"`
	UpdatedUnix int64  `json:"updated_unix"`
}

// SideDetail is everything ReadSideDetail loads from dogego_analytics.db when the store exists.
type SideDetail struct {
	Exists         bool
	Size           int64
	Schema         int
	RawBinCount    *int
	Meta           map[string]string
	IndexProgress  []IndexProgressRow
	MetricTimeline []MetricSample
	ReorgEvents    []ReorgEvent
	ReorgSummary   ReorgSummary
}

// ReadSideDetail reads dogego_analytics.db when present; if the store is missing it returns
// detail.Exists == false and nil error. On I/O or DB errors after the store exists, err is set.
func ReadSideDetail(dbPath string) (*SideDetail, error) {
	if !analyticsStoreExists(dbPath) {
		return &SideDetail{Exists: false}, nil
	}
	db, err := Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return readDetailFromOpenDB(db, dbPath)
}

func readDetailFromOpenDB(db *DB, dbPath string) (*SideDetail, error) {
	size, _ := DirSizeBytes(dbPath)
	d := &SideDetail{Exists: true, Size: size}
	d.Schema = SchemaVersion(db)

	rows, err := listIndexProgress(db)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Subsystem < rows[j].Subsystem })
	d.IndexProgress = rows
	for _, r := range rows {
		if r.Subsystem == "rawblocks" {
			v := int(r.LastHeight)
			d.RawBinCount = &v
		}
	}

	d.Meta, err = listMeta(db)
	if err != nil {
		return nil, err
	}
	d.MetricTimeline, err = ReadMetricSamples(db, maxMetricSamples)
	if err != nil {
		return nil, err
	}
	d.ReorgEvents, err = ReadReorgEvents(db, maxReorgEvents)
	if err != nil {
		return nil, err
	}
	d.ReorgSummary = SummarizeReorgEvents(d.ReorgEvents)
	return d, nil
}

// ReadSideSummary reads dogego_analytics.db if present (path is the Pebble directory).
// rawBinCount is set when a 'rawblocks' index_progress row exists (last_height stores bin count).
func ReadSideSummary(dbPath string) (exists bool, size int64, schema int, rawBinCount *int, err error) {
	d, err := ReadSideDetail(dbPath)
	if err != nil || d == nil {
		return false, 0, 0, nil, err
	}
	if !d.Exists {
		return false, 0, 0, nil, nil
	}
	return d.Exists, d.Size, d.Schema, d.RawBinCount, nil
}

// StoreExists reports whether an analytics Pebble directory exists at dbPath.
func StoreExists(dbPath string) bool {
	return analyticsStoreExists(dbPath)
}

// StoreSizeBytes returns on-disk size for an analytics Pebble directory (0 when missing).
func StoreSizeBytes(dbPath string) int64 {
	if !analyticsStoreExists(dbPath) {
		return 0
	}
	n, _ := DirSizeBytes(dbPath)
	return n
}
