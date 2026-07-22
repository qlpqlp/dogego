// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BlockCompressionStats summarizes on-disk block payload size vs uncompressed wire size.
type BlockCompressionStats struct {
	BlockCount          int
	StoredPayloadBytes  int64 // sum of block payloads on disk (records or plain .bin bodies)
	LogicalBytes        int64 // sum of uncompressed wire block sizes
	RawblocksDirBytes   int64 // entire rawblocks/ tree (locators, blk, overhead)
	CompressionRatio    float64 // stored/logical when logical > 0
	CompressionSavingsPct float64 // (1-ratio)*100 when ratio < 1
}

type compressionStatsCache struct {
	mu      sync.Mutex
	scanned time.Time
	ttl     time.Duration
	stats   BlockCompressionStats
	err     error
}

// CachedCompressionStats returns scan results, refreshing when older than ttl.
func (s *RawBlockStore) CachedCompressionStats(ttl time.Duration) (BlockCompressionStats, error) {
	if s == nil {
		return BlockCompressionStats{}, nil
	}
	if ttl <= 0 {
		ttl = 45 * time.Second
	}
	s.mu.Lock()
	cache := s.compStats
	if cache == nil {
		cache = &compressionStatsCache{ttl: ttl}
		s.compStats = cache
	}
	s.mu.Unlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()
	if time.Since(cache.scanned) < cache.ttl && cache.scanned.After(time.Time{}) {
		return cache.stats, cache.err
	}
	st, err := s.scanCompressionStats()
	cache.stats = st
	cache.err = err
	cache.scanned = time.Now()
	return st, err
}

// InvalidateCompressionStatsCache clears cached compression totals (after bulk import).
func (s *RawBlockStore) InvalidateCompressionStatsCache() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cache := s.compStats
	s.mu.Unlock()
	if cache == nil {
		return
	}
	cache.mu.Lock()
	cache.scanned = time.Time{}
	cache.mu.Unlock()
}

func (s *RawBlockStore) scanCompressionStats() (BlockCompressionStats, error) {
	s.mu.Lock()
	dir := s.dir
	opts := s.opts
	s.mu.Unlock()

	var st BlockCompressionStats
	locRoot := filepath.Join(dir, "loc")
	seenBin := make(map[string]struct{})

	err := filepath.Walk(locRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if len(name) != 64 {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		loc, err := decodeBlockLocator(b)
		if err != nil {
			return nil
		}
		st.BlockCount++
		if loc.Uncompressed > 0 {
			st.LogicalBytes += int64(loc.Uncompressed)
		}
		if loc.FileNum == perFileLocatorNum {
			hexName := name
			seenBin[hexName] = struct{}{}
			if fi, err := os.Stat(filepath.Join(dir, hexName+".bin")); err == nil {
				st.StoredPayloadBytes += fi.Size()
				if loc.Uncompressed == 0 {
					st.LogicalBytes += fi.Size()
				}
			}
		} else {
			st.StoredPayloadBytes += int64(loc.RecordLen)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return st, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return st, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".bin" {
			continue
		}
		hexName := strings.TrimSuffix(e.Name(), ".bin")
		if _, ok := seenBin[hexName]; ok {
			continue
		}
		b, err := hex.DecodeString(hexName)
		if err != nil || len(b) != 32 {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		st.BlockCount++
		if isPlainLegacyBin(data) {
			st.LogicalBytes += int64(len(data))
			st.StoredPayloadBytes += int64(len(data))
			continue
		}
		if len(data) >= blockRecordHeaderLen {
			logical := int64(binary.LittleEndian.Uint32(data[4:8]))
			st.LogicalBytes += logical
			st.StoredPayloadBytes += int64(len(data))
		}
	}

	st.RawblocksDirBytes, _ = dirSizeBytes(dir)
	if st.LogicalBytes > 0 {
		st.CompressionRatio = float64(st.StoredPayloadBytes) / float64(st.LogicalBytes)
		if st.CompressionRatio < 1 {
			st.CompressionSavingsPct = (1 - st.CompressionRatio) * 100
		}
	}
	_ = opts
	return st, nil
}

func dirSizeBytes(root string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}
