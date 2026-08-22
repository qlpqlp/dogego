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
	"time"
)

// legacyPerFileSubdir holds leftover hash.bin bodies after a perfile→bundled upgrade.
// Keeping them out of rawblocks/ root stops NTFS ReadDir/Stat storms on every Put.
const legacyPerFileSubdir = "legacy"

const (
	legacyMigrateBatch = 64
	legacyMigrateDelay = 3 * time.Minute // let body IBD claim the disk first after restart
	legacyMigrateYield = 50 * time.Millisecond
)

// StartLegacyPerFileBinMigration moves leftover *.bin out of rawblocks/ into rawblocks/legacy/
// when running bundled layout. Deferred so the first ReadDir of 200k files does not starve Puts.
func (s *RawBlockStore) StartLegacyPerFileBinMigration() {
	if s == nil {
		return
	}
	s.mu.Lock()
	layout := s.opts.Layout
	dir := s.dir
	s.mu.Unlock()
	if layout != BlockLayoutBundled || dir == "" {
		return
	}
	if !s.legacyMigrateStarted.CompareAndSwap(false, true) {
		return
	}
	go func() {
		time.Sleep(legacyMigrateDelay)
		s.migrateLegacyPerFileBins(dir)
	}()
}

// MigrateLegacyPerFileBinsNow runs the leftover *.bin move synchronously (tests / operator repair).
func (s *RawBlockStore) MigrateLegacyPerFileBinsNow() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	dir := s.dir
	layout := s.opts.Layout
	s.mu.Unlock()
	if layout != BlockLayoutBundled || dir == "" {
		return 0
	}
	s.legacyMigrateStarted.Store(true)
	return s.migrateLegacyPerFileBins(dir)
}

func (s *RawBlockStore) migrateLegacyPerFileBins(dir string) int {
	legacyDir := filepath.Join(dir, legacyPerFileSubdir)
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "rawblocks legacy migrate: mkdir: %v\n", err)
		return 0
	}
	// Prefer streaming the directory so we can move batches before the full listing finishes.
	f, err := os.Open(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rawblocks legacy migrate: open: %v\n", err)
		return 0
	}
	defer f.Close()

	var moved int64
	for {
		entries, readErr := f.ReadDir(legacyMigrateBatch * 4)
		if len(entries) == 0 {
			break
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !isLegacyHashBinName(name) {
				continue
			}
			src := filepath.Join(dir, name)
			dst := filepath.Join(legacyDir, name)
			if _, err := os.Stat(dst); err == nil {
				_ = os.Remove(src)
				moved++
				continue
			}
			if err := os.Rename(src, dst); err != nil {
				if cpErr := copyFileReplace(src, dst); cpErr != nil {
					continue
				}
				_ = os.Remove(src)
			}
			moved++
			if moved%64 == 0 {
				time.Sleep(legacyMigrateYield) // yield disk to bundled Put / getdata
			}
			if moved > 0 && moved%10000 == 0 {
				fmt.Fprintf(os.Stderr, "DogeGo: migrated %d leftover rawblocks/*.bin → rawblocks/%s/\n", moved, legacyPerFileSubdir)
			}
		}
		if readErr != nil {
			break
		}
	}
	if moved > 0 {
		fmt.Fprintf(os.Stderr, "DogeGo: migrated %d leftover rawblocks/*.bin → rawblocks/%s/ (hot dir clear for bundled IBD)\n", moved, legacyPerFileSubdir)
		s.InvalidateBytesOnDiskCache()
		// Do not Walk the leftover tree here — blk*.dat size is enough for the dashboard
		// until a quiet RefreshPayloadBytes runs later.
		if blk, err := s.sumBlkDatBytes(); err == nil {
			s.payloadBytes.Store(0) // leftover size unknown until background walk; blk tip is live
			_ = blk
			s.payloadBytesReady.Store(false)
		}
	}
	return int(moved)
}

func isLegacyHashBinName(name string) bool {
	if len(name) != 64+4 || !strings.HasSuffix(name, ".bin") {
		return false
	}
	for _, c := range name[:64] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func copyFileReplace(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, in, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func (s *RawBlockStore) legacyDir() string {
	if s == nil {
		return ""
	}
	return filepath.Join(s.dir, legacyPerFileSubdir)
}

func (s *RawBlockStore) legacyPathFor(hashLE [32]byte) string {
	name := hex.EncodeToString(hashLE[:])
	return filepath.Join(s.dir, legacyPerFileSubdir, name+".bin")
}

// resolvePerFilePath returns the on-disk path for a leftover/per-file body (root or legacy/).
func (s *RawBlockStore) resolvePerFilePath(hashLE [32]byte) (string, bool) {
	if s == nil {
		return "", false
	}
	root := s.pathFor(hashLE)
	if fi, err := os.Stat(root); err == nil && !fi.IsDir() {
		return root, true
	}
	legacy := s.legacyPathFor(hashLE)
	if fi, err := os.Stat(legacy); err == nil && !fi.IsDir() {
		return legacy, true
	}
	return root, false
}

func dirHasHashBin(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	for {
		entries, readErr := f.ReadDir(64)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if isLegacyHashBinName(e.Name()) {
				return true
			}
		}
		if readErr != nil || len(entries) == 0 {
			return false
		}
	}
}
