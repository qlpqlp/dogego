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

// RawBlockSyncCheckpoint persists progressive full-block download position (height probe).
type RawBlockSyncCheckpoint struct {
	NextProbeHeight     int64 `json:"next_probe_height"`
	ContiguousRawHeight int64 `json:"contiguous_raw_height,omitempty"` // monotonic body replay tip; omitted when unknown
}

const rawBlockSyncFile = "rawblocks_sync.json"

// PurgeStaleRawBlockSyncTemps removes crash leftovers from atomic checkpoint writes.
func PurgeStaleRawBlockSyncTemps(chainDir string) (int, error) {
	if chainDir == "" {
		return 0, nil
	}
	tmp := filepath.Join(chainDir, rawBlockSyncFile+".tmp")
	if _, err := os.Stat(tmp); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if err := os.Remove(tmp); err != nil {
		return 0, err
	}
	return 1, nil
}

// ReconcileRawBlockSyncCheckpoint clamps a stale checkpoint to the on-disk contiguous frontier (Core-style auto-heal).
func ReconcileRawBlockSyncCheckpoint(chainDir string, contiguous int64) (bool, error) {
	if chainDir == "" {
		return false, nil
	}
	changed := false
	if n, err := PurgeStaleRawBlockSyncTemps(chainDir); err != nil {
		return false, err
	} else if n > 0 {
		changed = true
	}
	path := filepath.Join(chainDir, rawBlockSyncFile)
	cp, err := LoadRawBlockSyncCheckpoint(chainDir)
	corrupt := false
	if err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return changed, removeErr
		}
		changed = true
		corrupt = true
		cp = RawBlockSyncCheckpoint{}
	}
	if contiguous < 0 {
		return changed, nil
	}
	wantProbe := contiguous + 1
	if wantProbe < 0 {
		wantProbe = 0
	}
	out := cp
	if corrupt || out.NextProbeHeight < 0 || out.NextProbeHeight > contiguous+1 {
		out.NextProbeHeight = wantProbe
		changed = true
	}
	if out.ContiguousRawHeight > contiguous {
		out.ContiguousRawHeight = contiguous
		changed = true
	} else if corrupt || (out.ContiguousRawHeight < 0 && contiguous >= 0) {
		out.ContiguousRawHeight = contiguous
		changed = true
	}
	if !changed {
		return false, nil
	}
	if err := SaveRawBlockSyncCheckpoint(chainDir, out); err != nil {
		return false, err
	}
	return true, nil
}

// LoadRawBlockSyncCheckpoint reads <chainDir>/rawblocks_sync.json (missing file → zero value, nil error).
func LoadRawBlockSyncCheckpoint(chainDir string) (RawBlockSyncCheckpoint, error) {
	var cp RawBlockSyncCheckpoint
	path := filepath.Join(chainDir, rawBlockSyncFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cp, nil
		}
		return cp, err
	}
	if err := json.Unmarshal(b, &cp); err != nil {
		return cp, err
	}
	return cp, nil
}

// SaveRawBlockSyncCheckpoint writes the checkpoint atomically.
func SaveRawBlockSyncCheckpoint(chainDir string, cp RawBlockSyncCheckpoint) error {
	path := filepath.Join(chainDir, rawBlockSyncFile)
	b, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b, 0o600)
}
