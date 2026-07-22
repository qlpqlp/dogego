// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	utxoSnapshotMagic   = "DGUT"
	utxoSnapshotVersion = uint32(1)
)

// UtxoSnapshotPath is the on-disk UTXO cache file under a chain data directory (not Core chainstate).
func UtxoSnapshotPath(chainRoot string) string {
	return filepath.Join(chainRoot, "utxo.cache")
}

// PurgeStaleUtxoSnapshotTemps removes incomplete utxo.cache.tmp left by interrupted SaveSnapshot.
func PurgeStaleUtxoSnapshotTemps(chainDir string) (int, error) {
	if chainDir == "" {
		return 0, nil
	}
	tmp := UtxoSnapshotPath(chainDir) + ".tmp"
	if err := os.Remove(tmp); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return 1, nil
}

// cloneUtxoCoinsForSnapshot copies the coin map under a brief read lock so disk I/O does not block connect.
func cloneUtxoCoinsForSnapshot(coins map[[36]byte]UtxoEntry) map[[36]byte]UtxoEntry {
	out := make(map[[36]byte]UtxoEntry, len(coins))
	for k, e := range coins {
		pk := e.PkScript
		if len(pk) > 0 {
			pk = append([]byte(nil), pk...)
		}
		out[k] = UtxoEntry{Value: e.Value, Height: e.Height, PkScript: pk}
	}
	return out
}

// SaveSnapshot writes the UTXO set to path (atomic replace via temp file).
func (u *UtxoCache) SaveSnapshot(path string) error {
	if u == nil {
		return fmt.Errorf("utxo snapshot: nil cache")
	}
	u.mu.RLock()
	tip := u.tipHeight
	coins := cloneUtxoCoinsForSnapshot(u.coins)
	u.mu.RUnlock()
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := writeUtxoSnapshot(f, tip, coins); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// LoadUtxoSnapshot loads a cache from path; returns nil, nil when the file is absent.
func LoadUtxoSnapshot(path string) (*UtxoCache, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	tip, coins, err := readUtxoSnapshot(f)
	if err != nil {
		return nil, err
	}
	return &UtxoCache{tipHeight: tip, coins: coins}, nil
}

func writeUtxoSnapshot(w io.Writer, tip int64, coins map[[36]byte]UtxoEntry) error {
	if _, err := io.WriteString(w, utxoSnapshotMagic); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, utxoSnapshotVersion); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, tip); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint64(len(coins))); err != nil {
		return err
	}
	for k, e := range coins {
		if _, err := w.Write(k[:]); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, e.Value); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, e.Height); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(e.PkScript))); err != nil {
			return err
		}
		if len(e.PkScript) > 0 {
			if _, err := w.Write(e.PkScript); err != nil {
				return err
			}
		}
	}
	return nil
}

func readUtxoSnapshot(r io.Reader) (tip int64, coins map[[36]byte]UtxoEntry, err error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return 0, nil, err
	}
	if string(magic[:]) != utxoSnapshotMagic {
		return 0, nil, fmt.Errorf("utxo snapshot: bad magic")
	}
	var ver uint32
	if err := binary.Read(r, binary.LittleEndian, &ver); err != nil {
		return 0, nil, err
	}
	if ver != utxoSnapshotVersion {
		return 0, nil, fmt.Errorf("utxo snapshot: unsupported version %d", ver)
	}
	if err := binary.Read(r, binary.LittleEndian, &tip); err != nil {
		return 0, nil, err
	}
	var n uint64
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return 0, nil, err
	}
	coins = make(map[[36]byte]UtxoEntry, n)
	for i := uint64(0); i < n; i++ {
		var k [36]byte
		if _, err := io.ReadFull(r, k[:]); err != nil {
			return 0, nil, err
		}
		var e UtxoEntry
		if err := binary.Read(r, binary.LittleEndian, &e.Value); err != nil {
			return 0, nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &e.Height); err != nil {
			return 0, nil, err
		}
		var slen uint32
		if err := binary.Read(r, binary.LittleEndian, &slen); err != nil {
			return 0, nil, err
		}
		if slen > 10_000 {
			return 0, nil, fmt.Errorf("utxo snapshot: script too large")
		}
		if slen > 0 {
			e.PkScript = make([]byte, slen)
			if _, err := io.ReadFull(r, e.PkScript); err != nil {
				return 0, nil, err
			}
		}
		coins[k] = e
	}
	return tip, coins, nil
}
