// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const persistFileVersion = 2
const persistFileVersionLegacy = 1

// PersistFileName is the DogeGo mempool dump under the chain data directory (not Core binary mempool.dat).
const PersistFileName = "dogego_mempool.json"

// PersistPath returns the absolute path for a chain data directory.
func PersistPath(chainDataDir string) string {
	return filepath.Join(chainDataDir, PersistFileName)
}

type persistFile struct {
	Version      int              `json:"version"`
	SavedAt      int64            `json:"saved_at"`
	Transactions []string         `json:"transactions"`
	FeeDeltas    map[string]int64 `json:"fee_deltas,omitempty"`
	DogegoNote   string           `json:"dogego_note,omitempty"`
}

// PersistSnapshot is a mempool dump payload (transactions + Core mapDeltas fee deltas).
type PersistSnapshot struct {
	Transactions [][]byte
	FeeDeltas    map[string]int64
}

// SavePersisted writes serialized transactions to path (atomic replace).
func SavePersisted(path string, rawTxs [][]byte, feeDeltas map[string]int64) error {
	return SavePersistedSnapshot(path, PersistSnapshot{Transactions: rawTxs, FeeDeltas: feeDeltas})
}

// SavePersistedSnapshot writes a full mempool dump (v2 when fee_deltas present).
func SavePersistedSnapshot(path string, snap PersistSnapshot) error {
	if path == "" {
		return fmt.Errorf("mempool persist: empty path")
	}
	hexes := make([]string, 0, len(snap.Transactions))
	for _, raw := range snap.Transactions {
		if len(raw) == 0 {
			continue
		}
		hexes = append(hexes, hex.EncodeToString(raw))
	}
	version := persistFileVersionLegacy
	if len(snap.FeeDeltas) > 0 {
		version = persistFileVersion
	}
	payload := persistFile{
		Version:      version,
		SavedAt:      time.Now().Unix(),
		Transactions: hexes,
		FeeDeltas:    snap.FeeDeltas,
		DogegoNote:   "DogeGo JSON mempool dump; fee_deltas mirrors Core mapDeltas; not compatible with Dogecoin Core mempool.dat",
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadPersisted reads serialized transactions from path. Missing file returns nil, nil.
func LoadPersisted(path string) ([][]byte, error) {
	snap, err := LoadPersistedSnapshot(path)
	if err != nil {
		return nil, err
	}
	return snap.Transactions, nil
}

// LoadPersistedSnapshot reads transactions and fee deltas from path.
func LoadPersistedSnapshot(path string) (PersistSnapshot, error) {
	if path == "" {
		return PersistSnapshot{}, fmt.Errorf("mempool persist: empty path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PersistSnapshot{}, nil
		}
		return PersistSnapshot{}, err
	}
	var pf persistFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return PersistSnapshot{}, fmt.Errorf("mempool persist: %w", err)
	}
	if pf.Version != 0 && pf.Version != persistFileVersionLegacy && pf.Version != persistFileVersion {
		return PersistSnapshot{}, fmt.Errorf("mempool persist: unsupported version %d", pf.Version)
	}
	out := make([][]byte, 0, len(pf.Transactions))
	for _, h := range pf.Transactions {
		raw, err := hex.DecodeString(h)
		if err != nil || len(raw) == 0 {
			continue
		}
		out = append(out, raw)
	}
	deltas := pf.FeeDeltas
	if len(deltas) == 0 {
		deltas = nil
	}
	return PersistSnapshot{Transactions: out, FeeDeltas: deltas}, nil
}
