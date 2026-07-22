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

const headerSyncFile = "headers_sync.json"

// PurgeStaleHeaderSyncTemps removes crash leftovers from atomic headers_sync.json writes.
func PurgeStaleHeaderSyncTemps(chainDir string) (int, error) {
	if chainDir == "" {
		return 0, nil
	}
	tmp := filepath.Join(chainDir, headerSyncFile+".tmp")
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

// HeaderSyncCheckpoint records the last fully committed header tip (Core-style crash recovery).
type HeaderSyncCheckpoint struct {
	Layout       string `json:"layout"` // "monolith" or "segments"
	TipHeight    int64  `json:"tip_height"`
	HeaderCount  int64  `json:"header_count"`
	JournalBytes int64  `json:"journal_bytes,omitempty"` // monolith only
	TipHashHex   string `json:"tip_hash_hex,omitempty"`
}

// LoadHeaderSyncCheckpoint reads chainDir/headers_sync.json (missing → zero, nil error).
func LoadHeaderSyncCheckpoint(chainDir string) (HeaderSyncCheckpoint, error) {
	var cp HeaderSyncCheckpoint
	path := filepath.Join(chainDir, headerSyncFile)
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

// SaveHeaderSyncCheckpoint writes the checkpoint atomically (tmp + rename).
func SaveHeaderSyncCheckpoint(chainDir string, cp HeaderSyncCheckpoint) error {
	path := filepath.Join(chainDir, headerSyncFile)
	b, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, b, 0o600)
}

// RepairMonolithFromCheckpoint truncates headers.bin to the last checkpoint when the file grew past it without a valid commit.
func RepairMonolithFromCheckpoint(chainDir, monolithPath string) (bool, error) {
	cp, err := LoadHeaderSyncCheckpoint(chainDir)
	if err != nil || cp.Layout != "monolith" || cp.HeaderCount <= 0 {
		return false, err
	}
	st, err := os.Stat(monolithPath)
	if err != nil {
		return false, err
	}
	want := cp.HeaderCount * 80
	if st.Size() <= want {
		return false, nil
	}
	if st.Size()%80 != 0 {
		keep := (st.Size() / 80) * 80
		if keep > want {
			keep = want
		}
		want = keep
	}
	f, err := os.OpenFile(monolithPath, os.O_RDWR, 0o600)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if err := f.Truncate(want); err != nil {
		return false, err
	}
	if err := f.Sync(); err != nil {
		return false, err
	}
	return true, nil
}
