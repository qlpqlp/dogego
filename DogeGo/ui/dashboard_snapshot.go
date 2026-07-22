// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	dashboardSnapshotFileName = "ui_dashboard.snapshot.json"
	dashboardSnapshotVersion  = 1
)

// dashboardSnapshotFile is the on-disk last-known dashboard payload for restart paint.
type dashboardSnapshotFile struct {
	Version           int            `json:"version"`
	SavedAt           string         `json:"saved_at"`
	TipHeight         int64          `json:"tip_height"`
	Summary           map[string]any `json:"summary"`
	P2P               json.RawMessage `json:"p2p,omitempty"`
	Mempool           json.RawMessage `json:"mempool,omitempty"`
	AnalyticsSummary  map[string]any `json:"analytics_summary,omitempty"`
}

func dashboardSnapshotPath(chainDataDir string) string {
	if chainDataDir == "" {
		return ""
	}
	return filepath.Join(chainDataDir, dashboardSnapshotFileName)
}

func loadDashboardSnapshot(chainDataDir string) (*dashboardSnapshotFile, error) {
	path := dashboardSnapshotPath(chainDataDir)
	if path == "" {
		return nil, os.ErrNotExist
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap dashboardSnapshotFile
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, err
	}
	if snap.Summary == nil || len(snap.Summary) == 0 {
		return nil, os.ErrInvalid
	}
	return &snap, nil
}

func saveDashboardSnapshot(chainDataDir string, snap *dashboardSnapshotFile) error {
	if snap == nil || chainDataDir == "" {
		return nil
	}
	if err := os.MkdirAll(chainDataDir, 0o755); err != nil {
		return err
	}
	path := dashboardSnapshotPath(chainDataDir)
	snap.Version = dashboardSnapshotVersion
	if snap.SavedAt == "" {
		snap.SavedAt = time.Now().UTC().Format(time.RFC3339)
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return atomicWriteUIFile(path, b)
}

func atomicWriteUIFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			_ = os.WriteFile(path, data, 0o644)
			_ = os.Remove(tmp)
			return err2
		}
	}
	return nil
}

func tipHeightFromSummary(sum map[string]any) int64 {
	if sum == nil {
		return -1
	}
	switch v := sum["tip_height"].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return -1
	}
}

func markSummaryFromDiskSnapshot(sum map[string]any) {
	if sum == nil {
		return
	}
	sum["summary_stale"] = true
	sum["from_disk_snapshot"] = true
	sum["dogego_ui_loading"] = false
	delete(sum, "dogego_ui_loading_phase")
	delete(sum, "dogego_ui_loading_detail")
	sum["sync_status_line"] = "Showing last known data · refreshing…"
	sum["dogego_sync_status"] = sum["sync_status_line"]
}

func clearSummaryStaleFlags(sum map[string]any) {
	if sum == nil {
		return
	}
	delete(sum, "summary_stale")
	delete(sum, "from_disk_snapshot")
}
