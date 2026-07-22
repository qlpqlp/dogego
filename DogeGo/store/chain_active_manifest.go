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
	"time"
)

// ChainActiveManifestPath is the sidecar JSON written with utxo.cache (chain connect checkpoint).
func ChainActiveManifestPath(chainRoot string) string {
	return filepath.Join(chainRoot, "chain_active.manifest.json")
}

// ChainActiveManifest records chainActive tip and body coverage at last utxo.cache save.
type ChainActiveManifest struct {
	UtxoTipHeight       int64  `json:"utxo_tip_height"`
	UtxoTipBlockHash    string `json:"utxo_tip_block_hash,omitempty"`
	ContiguousRawHeight int64  `json:"contiguous_raw_height"`
	SavedAtUnix         int64  `json:"saved_at_unix"`
}

// SaveChainActiveManifest writes the connect checkpoint (atomic replace).
func SaveChainActiveManifest(chainRoot string, m ChainActiveManifest) error {
	if chainRoot == "" {
		return nil
	}
	if m.SavedAtUnix == 0 {
		m.SavedAtUnix = time.Now().Unix()
	}
	path := ChainActiveManifestPath(chainRoot)
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadChainActiveManifest reads chain_active.manifest.json; nil when absent.
func LoadChainActiveManifest(chainRoot string) (*ChainActiveManifest, error) {
	path := ChainActiveManifestPath(chainRoot)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m ChainActiveManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
