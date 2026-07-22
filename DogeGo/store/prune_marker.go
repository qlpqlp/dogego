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
)

// PruneMarker records the lowest height kept after operator pruneblockchain (Core pruned node).
type PruneMarker struct {
	PruneHeight int64 `json:"prune_height"`
}

func pruneMarkerPath(chainDir string) string {
	return filepath.Join(chainDir, "prune_marker.json")
}

// SavePruneMarker persists prune_height after pruneblockchain removes old raw blocks.
func SavePruneMarker(chainDir string, pruneHeight int64) error {
	if chainDir == "" || pruneHeight < 0 {
		return nil
	}
	b, err := json.Marshal(PruneMarker{PruneHeight: pruneHeight})
	if err != nil {
		return err
	}
	tmp := pruneMarkerPath(chainDir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, pruneMarkerPath(chainDir))
}

// LoadPruneMarker returns the stored prune height when the marker file exists.
func LoadPruneMarker(chainDir string) (height int64, ok bool) {
	if chainDir == "" {
		return 0, false
	}
	b, err := os.ReadFile(pruneMarkerPath(chainDir))
	if err != nil {
		return 0, false
	}
	var m PruneMarker
	if err := json.Unmarshal(b, &m); err != nil || m.PruneHeight < 0 {
		return 0, false
	}
	return m.PruneHeight, true
}
