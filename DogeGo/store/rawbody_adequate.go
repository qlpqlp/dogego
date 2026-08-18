// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"

	"dogego/chain"
	"dogego/pow"
)

// MainnetGenesisStubTestBytes is a deliberately undersized raw block file used in purge/recovery tests
// (below MinRawBlockBytes at height 0; real mainnet genesis is ~224 B).
const MainnetGenesisStubTestBytes = 190

// MinRawBlockBytes is the minimum stored payload size for a height to count as downloaded.
// Real mainnet coinbase-only blocks are often ~190-250 B through early IBD (e.g. height 10006 is 213 B per Core);
// consensus validation rejects pruned-peer stubs; size is only a coarse coverage gate.
func MinRawBlockBytes(net chain.Network, height int64) int {
	if height == 0 {
		// Mainnet genesis is ~224 B (below the early-block floor); test stubs are ~190 B.
		if net == chain.MainnetDogecoin {
			return 200
		}
		return 80
	}
	if height < 0 {
		return 80
	}
	if net == chain.MainnetDogecoin && height < 500_000 {
		return 140
	}
	return 80
}

// HasStoredBody reports whether rawblocks/ has an adequate payload for hashLE.
// Uses locator Uncompressed / file Stat size — never a full Get — so IBD claim planning stays cheap.
func (s *RawBlockStore) HasStoredBody(hashLE [32]byte, minBytes int) bool {
	if s == nil {
		return false
	}
	if minBytes <= 0 {
		minBytes = 80
	}
	size, ok := s.storedPayloadSize(hashLE)
	if !ok {
		return false
	}
	return size >= minBytes
}

// storedPayloadSize returns the uncompressed (or on-disk) payload length without reading the body.
func (s *RawBlockStore) storedPayloadSize(hashLE [32]byte) (int, bool) {
	if s.writeBehind != nil {
		if n, ok := s.writeBehind.size(hashLE); ok {
			return n, true
		}
	}
	path := s.pathFor(hashLE)
	s.mu.Lock()
	loc, ok, err := readBlockLocator(s.locatorRoot(), hashLE)
	s.mu.Unlock()
	if err == nil && ok {
		if loc.FileNum == perFileLocatorNum {
			fi, err := os.Stat(path)
			if err != nil {
				return 0, false
			}
			// Zstd per-file: locator Uncompressed is the decoded payload size.
			if loc.Uncompressed > 0 {
				return int(loc.Uncompressed), true
			}
			return int(fi.Size()), true
		}
		// Bundled: prefer Uncompressed; RecordLen is the on-disk record (may be compressed).
		if loc.Uncompressed > 0 {
			return int(loc.Uncompressed), true
		}
		if loc.RecordLen > 0 {
			return int(loc.RecordLen), true
		}
		return 0, false
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	return int(fi.Size()), true
}

// HasStoredBodyAtHeight checks the journal header hash at height.
func HasStoredBodyAtHeight(j *HeaderJournal, raw *RawBlockStore, height int64, net chain.Network) bool {
	if j == nil || raw == nil || height < 0 {
		return false
	}
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return false
	}
	return raw.HasStoredBody(pow.BlockHashLE(h80), MinRawBlockBytes(net, height))
}
