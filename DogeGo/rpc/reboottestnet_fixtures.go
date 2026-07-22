// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dogego/store"
)

func rebootTestnetFixtureRelPath(height int) string {
	return filepath.Join("testdata", "reboottestnet_mined", fmt.Sprintf("block%d.hex", height))
}

func rebootTestnetFixtureSearchBases() []string {
	return nil
}

func findRebootTestnetFixturePath(rel string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		for _, cand := range []string{
			filepath.Join(dir, rel),
			filepath.Join(dir, "rpc", rel),
			filepath.Join(dir, "DogeGo", "rpc", rel),
		} {
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("not found")
}

// RebootTestnetMinedFixture returns a pre-mined legacy block payload at the given height (1..3).
func RebootTestnetMinedFixture(height int) ([]byte, error) {
	if height < 1 || height > 3 {
		return nil, fmt.Errorf("fixture height %d out of range 1..3", height)
	}
	rel := rebootTestnetFixtureRelPath(height)
	path, err := findRebootTestnetFixturePath(rel)
	if err != nil {
		return nil, fmt.Errorf("reboot testnet mined fixture %s: %w (regenerate with DOGEGO_GEN_REBOOT_FIXTURES=1)", rel, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	payload, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, err
	}
	if len(payload) < 80 {
		return nil, fmt.Errorf("fixture %s too short (%d bytes)", path, len(payload))
	}
	return payload, nil
}

// AppendRebootTestnetMinedFixture appends a committed scrypt-mined header at height to the journal.
func AppendRebootTestnetMinedFixture(j *store.HeaderJournal, height int) ([]byte, error) {
	payload, err := RebootTestnetMinedFixture(height)
	if err != nil {
		return nil, err
	}
	if err := j.AppendHeaders([][]byte{payload[:80]}); err != nil {
		return nil, err
	}
	return payload[:80], nil
}
