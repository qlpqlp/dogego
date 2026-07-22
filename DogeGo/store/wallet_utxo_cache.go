// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

)

type walletUtxoCacheFile struct {
	UtxoTipHeight int64         `json:"utxo_tip_height"`
	BestBlockHex  string        `json:"best_block_hex,omitempty"`
	ScriptsKey    string        `json:"scripts_key"`
	Rows          []UtxoDumpRow `json:"rows"`
}

// WalletScriptsKey fingerprints tracked wallet scripts for cache invalidation.
func WalletScriptsKey(scripts [][]byte) string {
	return walletScriptsKey(scripts)
}

// WalletUtxoCachePath is the persisted wallet UTXO scan cache under the chain datadir.
func WalletUtxoCachePath(chainRoot string) string {
	return filepath.Join(chainRoot, "wallet_utxo_scan.cache.json")
}

func walletScriptsKey(scripts [][]byte) string {
	if len(scripts) == 0 {
		return ""
	}
	cp := make([][]byte, len(scripts))
	for i, s := range scripts {
		cp[i] = append([]byte(nil), s...)
	}
	sort.Slice(cp, func(i, j int) bool { return string(cp[i]) < string(cp[j]) })
	h := sha256.New()
	for _, s := range cp {
		_, _ = h.Write([]byte{byte(len(s))})
		_, _ = h.Write(s)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// LoadWalletUtxoCache returns persisted wallet UTXO rows when tip height and scripts match.
func LoadWalletUtxoCache(path string, utxoTip int64, scriptsKey string) ([]UtxoDumpRow, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var f walletUtxoCacheFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, false
	}
	if f.ScriptsKey != scriptsKey {
		return nil, false
	}
	if f.UtxoTipHeight >= 0 {
		if f.UtxoTipHeight != utxoTip {
			return nil, false
		}
		return f.Rows, true
	}
	// Legacy cache keyed by best block hash only - caller must validate hash separately.
	return nil, false
}

// SaveWalletUtxoCache atomically writes filtered wallet UTXO rows for restart-fast balance queries.
func SaveWalletUtxoCache(path string, utxoTip int64, scriptsKey string, rows []UtxoDumpRow) error {
	f := walletUtxoCacheFile{
		UtxoTipHeight: utxoTip,
		ScriptsKey:    scriptsKey,
		Rows:          rows,
	}
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// InvalidateWalletUtxoCache removes the scan cache (after wallet import or chain rewind).
func InvalidateWalletUtxoCache(chainRoot string) error {
	path := WalletUtxoCachePath(chainRoot)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
