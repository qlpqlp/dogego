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

// LikelyHasBody reports whether a body is already staged in RAM or recorded in the locator
// without Stat'ing the payload file. Used during download-first claim planning so lanes do
// not re-getdata heights already stored ahead of the contiguous hole (NTFS Stat was too
// expensive to call per height, but blind skipDisk re-claimed disk-present orphans).
func (s *RawBlockStore) LikelyHasBody(hashLE [32]byte, minBytes int) bool {
	if s == nil {
		return false
	}
	if minBytes <= 0 {
		minBytes = 80
	}
	if s.writeBehind != nil {
		if n, ok := s.writeBehind.size(hashLE); ok && n >= minBytes {
			return true
		}
	}
	s.mu.Lock()
	loc, ok, err := readBlockLocator(s.locatorRoot(), hashLE)
	s.mu.Unlock()
	if err != nil || !ok {
		// Claim planning must not Stat leftover hash.bin under a 200k-file hybrid tree.
		// Heights already covered sit at/below contiguous tip (skipped via contSkip).
		// Ahead orphans without a locator are cheap to re-getdata; fetch skips if present.
		return false
	}
	if loc.Uncompressed >= uint32(minBytes) {
		return true
	}
	if loc.RecordLen >= uint32(minBytes) {
		return true
	}
	// Per-file locators may omit Uncompressed; presence still means Put already succeeded.
	return loc.FileNum == perFileLocatorNum
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
	s.mu.Lock()
	loc, ok, err := readBlockLocator(s.locatorRoot(), hashLE)
	s.mu.Unlock()
	if err == nil && ok {
		if loc.FileNum == perFileLocatorNum {
			p, found := s.resolvePerFilePath(hashLE)
			if !found {
				return 0, false
			}
			fi, err := os.Stat(p)
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
	p, found := s.resolvePerFilePath(hashLE)
	if !found {
		return 0, false
	}
	fi, err := os.Stat(p)
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
