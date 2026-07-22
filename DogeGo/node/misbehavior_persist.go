// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// misbehaviorFile is persisted per-chain misbehavior scores (Core ban scores subset).
type misbehaviorFile struct {
	Scores map[string]int `json:"scores"`
}

// LoadMisbehaviorScores merges scores from path into the tracker (expired bans are not stored here).
func LoadMisbehaviorScores(m *MisbehaviorTracker, path string) {
	if m == nil || path == "" {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var f misbehaviorFile
	if err := json.Unmarshal(b, &f); err != nil || len(f.Scores) == 0 {
		return
	}
	m.mu.Lock()
	for k, v := range f.Scores {
		if v > 0 {
			if m.scores == nil {
				m.scores = make(map[string]int)
			}
			m.scores[k] = v
		}
	}
	m.mu.Unlock()
}

// SaveMisbehaviorScores writes current scores to path (best-effort).
func SaveMisbehaviorScores(m *MisbehaviorTracker, path string) error {
	if m == nil || path == "" {
		return nil
	}
	m.mu.Lock()
	scores := make(map[string]int, len(m.scores))
	for k, v := range m.scores {
		if v > 0 {
			scores[k] = v
		}
	}
	m.mu.Unlock()
	if len(scores) == 0 {
		_ = os.Remove(path)
		return nil
	}
	b, err := json.MarshalIndent(misbehaviorFile{Scores: scores}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
