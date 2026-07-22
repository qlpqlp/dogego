// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// MainnetFieldAuxpowEntry is a committed auxpow proof for offline field validation (no datadir).
type MainnetFieldAuxpowEntry struct {
	Height    int64  `json:"height"`
	HeaderHex string `json:"header_hex"`
	AuxHex    string `json:"aux_hex"`
}

func mainnetFieldAuxpowFixturePath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("testdata", "mainnet_field_auxpow.json")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "mainnet_field_auxpow.json")
}

// LoadMainnetFieldAuxpowEntries reads consensus/testdata/mainnet_field_auxpow.json.
func LoadMainnetFieldAuxpowEntries() ([]MainnetFieldAuxpowEntry, error) {
	raw, err := os.ReadFile(mainnetFieldAuxpowFixturePath())
	if err != nil {
		return nil, err
	}
	var out []MainnetFieldAuxpowEntry
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("mainnet_field_auxpow.json empty")
	}
	return out, nil
}

// CommittedAuxpowHeaderHex returns header80 hex from mainnet_field_auxpow.json when present.
func CommittedAuxpowHeaderHex(height int64) (string, bool) {
	entries, err := LoadMainnetFieldAuxpowEntries()
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.Height == height {
			hx := strings.TrimSpace(e.HeaderHex)
			if hx != "" {
				return strings.ToUpper(hx), true
			}
			return "", false
		}
	}
	return "", false
}
