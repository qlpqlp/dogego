// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"os"
	"path/filepath"

	"dogego/wire"
)

// TxIndexFormatStats counts legacy (36-byte) and v2 (>36-byte) txid files under indexes/tx.
func (x *TxIndex) FormatStats() (legacy, v2 int, err error) {
	if x == nil {
		return 0, 0, fmt.Errorf("nil tx index")
	}
	x.mu.Lock()
	root := x.root
	x.mu.Unlock()
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) != 64 || !validTxIndexFileName(name) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		switch {
		case fi.Size() == txIndexMetaLen:
			legacy++
		case fi.Size() > txIndexMetaLen:
			v2++
		}
	}
	return legacy, v2, nil
}

func validTxIndexFileName(name string) bool {
	for _, c := range name {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

// UpgradeLegacyEntries rewrites up to maxFiles legacy index files with embedded serialized tx (v2).
func (x *TxIndex) UpgradeLegacyEntries(raw *RawBlockStore, maxFiles int) (upgraded int, err error) {
	if x == nil {
		return 0, fmt.Errorf("nil tx index")
	}
	if !x.EmbedTx {
		return 0, nil
	}
	if raw == nil {
		return 0, fmt.Errorf("nil raw block store")
	}
	if maxFiles <= 0 {
		return 0, nil
	}
	x.mu.Lock()
	root := x.root
	x.mu.Unlock()
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if upgraded >= maxFiles {
			break
		}
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) != 64 || !validTxIndexFileName(name) {
			continue
		}
		fi, err := e.Info()
		if err != nil || fi.Size() != txIndexMetaLen {
			continue
		}
		path := filepath.Join(root, name)
		if ok, err := x.upgradeLegacyFile(path, raw); err != nil {
			return upgraded, err
		} else if ok {
			upgraded++
		}
	}
	return upgraded, nil
}

func (x *TxIndex) upgradeLegacyFile(path string, raw *RawBlockStore) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if len(data) != txIndexMetaLen {
		return false, nil
	}
	hit, err := decodeTxIndexEntry(data)
	if err != nil {
		return false, err
	}
	payload, err := raw.Get(hit.BlockHashLE)
	if err != nil {
		return false, nil
	}
	tx, _, err := wire.ReadTxAtIndex(payload, hit.TxIndex)
	if err != nil {
		return false, nil
	}
	ser, err := tx.Serialize()
	if err != nil {
		return false, err
	}
	if len(ser) > 4_000_000 {
		return false, fmt.Errorf("tx too large for index %d bytes", len(ser))
	}
	entry := encodeTxIndexEntry(hit.BlockHashLE, hit.TxIndex, ser)
	x.mu.Lock()
	err = os.WriteFile(path, entry, 0o600)
	x.mu.Unlock()
	if err != nil {
		return false, err
	}
	return true, nil
}

// UpgradeLegacyTxIndexBatch upgrades legacy tx index files under chainDir (idempotent).
func UpgradeLegacyTxIndexBatch(chainDir string, maxFiles int) (upgraded int, legacyRemaining int, err error) {
	txIx, err := OpenTxIndex(chainDir)
	if err != nil {
		return 0, 0, err
	}
	raw, err := OpenRawBlockStore(chainDir)
	if err != nil {
		return 0, 0, err
	}
	upgraded, err = txIx.UpgradeLegacyEntries(raw, maxFiles)
	if err != nil {
		return upgraded, 0, err
	}
	legacy, _, err := txIx.FormatStats()
	if err != nil {
		return upgraded, 0, err
	}
	return upgraded, legacy, nil
}
