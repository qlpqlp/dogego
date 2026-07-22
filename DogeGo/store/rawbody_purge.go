// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dogego/chain"
	"dogego/pow"
)

// PurgeInadequateRawBodies removes raw block entries that exist on disk but cannot be read as
// adequate payloads (undersized per-file stubs, bundled locator hash mismatches, corrupt records).
// lowestRemoved is the journal height of the earliest removed body, or -1 when none removed.
func PurgeInadequateRawBodies(j *HeaderJournal, raw *RawBlockStore, net chain.Network) (purged int, lowestRemoved int64, err error) {
	if raw == nil {
		return 0, -1, nil
	}
	lowestRemoved = -1
	dir := raw.Dir()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return 0, -1, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".bin") {
			continue
		}
		hexName := strings.TrimSuffix(e.Name(), ".bin")
		b, err := hex.DecodeString(hexName)
		if err != nil || len(b) != 32 {
			continue
		}
		var hashLE [32]byte
		copy(hashLE[:], b)
		if removed, at, err := purgeRawBodyIfUnreadable(j, raw, hashLE, net); err != nil {
			return purged, lowestRemoved, err
		} else if removed {
			purged++
			lowestRemoved = minRemovedHeight(lowestRemoved, at)
		}
	}
	locRoot := filepath.Join(dir, "loc")
	locEntries, err := os.ReadDir(locRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return purged, lowestRemoved, nil
		}
		return purged, lowestRemoved, err
	}
	for _, prefix := range locEntries {
		if !prefix.IsDir() {
			continue
		}
		sub, err := os.ReadDir(filepath.Join(locRoot, prefix.Name()))
		if err != nil {
			return purged, lowestRemoved, err
		}
		for _, e := range sub {
			if e.IsDir() || len(e.Name()) != 64 {
				continue
			}
			b, err := hex.DecodeString(e.Name())
			if err != nil || len(b) != 32 {
				continue
			}
			var hashLE [32]byte
			copy(hashLE[:], b)
			if removed, at, err := purgeRawBodyIfUnreadable(j, raw, hashLE, net); err != nil {
				return purged, lowestRemoved, err
			} else if removed {
				purged++
				lowestRemoved = minRemovedHeight(lowestRemoved, at)
			}
		}
	}
	return purged, lowestRemoved, nil
}

// PurgeInadequateRawBodiesThroughHeight removes unreadable bodies for journal heights [0, through].
// Used at startup when a UTXO snapshot is ahead of stored bodies (reliable vs locator-only scans).
func PurgeInadequateRawBodiesThroughHeight(j *HeaderJournal, raw *RawBlockStore, through int64, net chain.Network) (purged int, lowestRemoved int64, err error) {
	if raw == nil || j == nil || through < 0 {
		return 0, -1, nil
	}
	lowestRemoved = -1
	for h := int64(0); h <= through; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			break
		}
		hashLE := pow.BlockHashLE(h80)
		if removed, at, err := purgeRawBodyIfUnreadable(j, raw, hashLE, net); err != nil {
			return purged, lowestRemoved, err
		} else if removed {
			purged++
			if at < 0 {
				at = h
			}
			lowestRemoved = minRemovedHeight(lowestRemoved, at)
		}
	}
	return purged, lowestRemoved, nil
}

func minRemovedHeight(lowest, at int64) int64 {
	if at < 0 {
		return lowest
	}
	if lowest < 0 || at < lowest {
		return at
	}
	return lowest
}

func purgeRawBodyIfUnreadable(j *HeaderJournal, raw *RawBlockStore, hashLE [32]byte, net chain.Network) (bool, int64, error) {
	if raw == nil || !raw.Has(hashLE) {
		return false, -1, nil
	}
	height := int64(-1)
	if j != nil {
		if h, herr := j.HeightByBlockHashLE(hashLE); herr == nil {
			height = h
		}
	}
	min := MinRawBlockBytes(net, height)
	if raw.HasStoredBody(hashLE, min) {
		return false, -1, nil
	}
	if err := raw.Remove(hashLE); err != nil {
		return false, -1, fmt.Errorf("remove %x: %w", hashLE[:4], err)
	}
	return true, height, nil
}
