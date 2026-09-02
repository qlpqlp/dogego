// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// blockLocator is a fixed-size on-disk pointer into rawblocks/ (bundled or per-file+zstd).
// 21 bytes: fileNum u32 | offset u64 | recordLen u32 | uncompressed u32 | flags u8

const blockLocatorLen = 21

const blockLocatorFlagZstd = 1 << 0

type blockLocator struct {
	FileNum      uint32
	Offset       uint64
	RecordLen    uint32
	Uncompressed uint32
	Flags        uint8
}

func encodeBlockLocator(loc blockLocator) [blockLocatorLen]byte {
	var b [blockLocatorLen]byte
	binary.LittleEndian.PutUint32(b[0:4], loc.FileNum)
	binary.LittleEndian.PutUint64(b[4:12], loc.Offset)
	binary.LittleEndian.PutUint32(b[12:16], loc.RecordLen)
	binary.LittleEndian.PutUint32(b[16:20], loc.Uncompressed)
	b[20] = loc.Flags
	return b
}

func decodeBlockLocator(b []byte) (blockLocator, error) {
	var loc blockLocator
	if len(b) != blockLocatorLen {
		return loc, fmt.Errorf("locator len %d", len(b))
	}
	loc.FileNum = binary.LittleEndian.Uint32(b[0:4])
	loc.Offset = binary.LittleEndian.Uint64(b[4:12])
	loc.RecordLen = binary.LittleEndian.Uint32(b[12:16])
	loc.Uncompressed = binary.LittleEndian.Uint32(b[16:20])
	loc.Flags = b[20]
	return loc, nil
}

func blockLocatorPath(locatorRoot string, hashLE [32]byte) string {
	hexName := hex.EncodeToString(hashLE[:])
	return filepath.Join(locatorRoot, hexName[:2], hexName)
}

func writeBlockLocator(locatorRoot string, hashLE [32]byte, loc blockLocator) error {
	path := blockLocatorPath(locatorRoot, hashLE)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload := encodeBlockLocator(loc)
	// Direct write: locators are 21 bytes and rebuildable from blk*.dat. Avoiding
	// tmp+rename halves NTFS create/rename traffic during IBD (was capping ~50 loc/min).
	return os.WriteFile(path, payload[:], 0o600)
}

func readBlockLocator(locatorRoot string, hashLE [32]byte) (blockLocator, bool, error) {
	path := blockLocatorPath(locatorRoot, hashLE)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return blockLocator{}, false, nil
		}
		return blockLocator{}, false, err
	}
	loc, err := decodeBlockLocator(b)
	if err != nil {
		return blockLocator{}, false, err
	}
	return loc, true, nil
}

func removeBlockLocator(locatorRoot string, hashLE [32]byte) error {
	path := blockLocatorPath(locatorRoot, hashLE)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func bundledBlkPath(rawDir string, fileNum uint32) string {
	return filepath.Join(rawDir, fmt.Sprintf("blk%05d.dat", fileNum))
}

func countBlockLocators(locatorRoot string) (int, error) {
	if _, err := os.Stat(locatorRoot); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	err := filepath.Walk(locatorRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if len(name) != 64 {
			return nil
		}
		if _, decErr := hex.DecodeString(name); decErr != nil {
			return nil
		}
		n++
		return nil
	})
	return n, err
}

// countLegacyLocatorsNotInJournal counts per-hash locator files whose hash is not already
// present in the append journal (upgrade / mixed trees).
func countLegacyLocatorsNotInJournal(locatorRoot string, lm *locatorMem) (int, error) {
	if lm == nil {
		return countBlockLocators(locatorRoot)
	}
	if _, err := os.Stat(locatorRoot); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	err := filepath.Walk(locatorRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if len(name) != 64 {
			return nil
		}
		b, decErr := hex.DecodeString(name)
		if decErr != nil || len(b) != 32 {
			return nil
		}
		var hash [32]byte
		copy(hash[:], b)
		if _, ok := lm.get(hash); ok {
			return nil
		}
		n++
		return nil
	})
	return n, err
}
