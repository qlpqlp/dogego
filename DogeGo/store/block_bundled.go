// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const maxBundledFileBytes = 128 * 1024 * 1024

func (s *RawBlockStore) locatorRoot() string {
	return filepath.Join(s.dir, "loc")
}

func (s *RawBlockStore) putBundled(hashLE [32]byte, raw []byte) error {
	rec, err := encodeBlockRecord(hashLE, raw, s.opts.Zstd)
	if err != nil {
		return err
	}
	s.bundledAppendMu.Lock()
	loc, err := s.appendBundledRecordLocked(rec, uint32(len(raw)))
	s.bundledAppendMu.Unlock()
	if err != nil {
		return err
	}
	return writeBlockLocator(s.locatorRoot(), hashLE, loc)
}

func (s *RawBlockStore) appendBundledRecordLocked(rec []byte, uncompressed uint32) (blockLocator, error) {
	fileNum, offset, err := s.pickBundledAppendSlot(int64(len(rec)))
	if err != nil {
		return blockLocator{}, err
	}
	f, err := s.openBundledAppendLocked(fileNum, offset)
	if err != nil {
		return blockLocator{}, err
	}
	if _, err := f.Write(rec); err != nil {
		_ = s.closeBundledAppendLocked()
		return blockLocator{}, err
	}
	// Do not fsync every block (Core buffers blk*.dat writes). Sync only when rotating
	// to a new file so IBD is not disk-bound on slow HDDs/Windows.
	if offset == 0 {
		if err := f.Sync(); err != nil {
			_ = s.closeBundledAppendLocked()
			return blockLocator{}, err
		}
	}
	s.noteBundledAppend(fileNum, offset, len(rec))
	if testing.Testing() {
		// Tests delete TempDir while the process still holds the handle on Windows.
		_ = s.closeBundledAppendLocked()
	}
	flags := uint8(0)
	if s.opts.Zstd {
		flags |= blockLocatorFlagZstd
	}
	return blockLocator{
		FileNum:      fileNum,
		Offset:       uint64(offset),
		RecordLen:    uint32(len(rec)),
		Uncompressed: uncompressed,
		Flags:        flags,
	}, nil
}

func (s *RawBlockStore) openBundledAppendLocked(fileNum uint32, offset int64) (*os.File, error) {
	if s.bundledFile != nil && s.bundledFileNum == fileNum {
		if _, err := s.bundledFile.Seek(offset, 0); err != nil {
			_ = s.closeBundledAppendLocked()
		} else {
			return s.bundledFile, nil
		}
	}
	_ = s.closeBundledAppendLocked()
	path := bundledBlkPath(s.dir, fileNum)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(offset, 0); err != nil {
		_ = f.Close()
		return nil, err
	}
	s.bundledFile = f
	s.bundledFileNum = fileNum
	return f, nil
}

func (s *RawBlockStore) closeBundledAppendLocked() error {
	if s.bundledFile == nil {
		return nil
	}
	err := s.bundledFile.Close()
	s.bundledFile = nil
	return err
}

// Close releases the cached bundled append handle (tests / shutdown).
func (s *RawBlockStore) Close() error {
	if s == nil {
		return nil
	}
	s.bundledAppendMu.Lock()
	err := s.closeBundledAppendLocked()
	s.bundledAppendMu.Unlock()
	return err
}

func (s *RawBlockStore) pickBundledAppendSlot(need int64) (fileNum uint32, offset int64, err error) {
	if need < 0 {
		need = 0
	}
	if s.bundledTipValid {
		if s.bundledTipSize+need <= maxBundledFileBytes {
			return s.bundledTipNum, s.bundledTipSize, nil
		}
		return s.bundledTipNum + 1, 0, nil
	}
	// Probe blk00000.dat, blk00001.dat, … only. Never ReadDir(rawblocks/) — after a
	// perfile→bundled upgrade that directory can hold 200k+ leftover *.bin files and a
	// single ReadDir stalls every Put (live: 0 blk/min with peers still in-flight).
	var tipNum uint32
	var tipSize int64 = -1
	for n := uint32(0); n < 100000; n++ {
		fi, stErr := os.Stat(bundledBlkPath(s.dir, n))
		if stErr != nil {
			if os.IsNotExist(stErr) {
				break
			}
			return 0, 0, stErr
		}
		tipNum = n
		tipSize = fi.Size()
	}
	s.bundledTipValid = true
	if tipSize < 0 {
		s.bundledTipNum = 0
		s.bundledTipSize = 0
		return 0, 0, nil
	}
	s.bundledTipNum = tipNum
	s.bundledTipSize = tipSize
	if tipSize+need <= maxBundledFileBytes {
		return tipNum, tipSize, nil
	}
	return tipNum + 1, 0, nil
}

func (s *RawBlockStore) noteBundledAppend(fileNum uint32, offset int64, recordLen int) {
	next := offset + int64(recordLen)
	if !s.bundledTipValid || fileNum > s.bundledTipNum {
		s.bundledTipValid = true
		s.bundledTipNum = fileNum
		s.bundledTipSize = next
		return
	}
	if fileNum == s.bundledTipNum && next > s.bundledTipSize {
		s.bundledTipSize = next
	}
}

func (s *RawBlockStore) getViaLocator(hashLE [32]byte) ([]byte, error) {
	loc, ok, err := readBlockLocator(s.locatorRoot(), hashLE)
	if err != nil || !ok {
		return nil, err
	}
	if loc.FileNum == perFileLocatorNum {
		return s.readPerFileLocator(hashLE, loc)
	}
	path := bundledBlkPath(s.dir, loc.FileNum)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rec := make([]byte, loc.RecordLen)
	if _, err := f.ReadAt(rec, int64(loc.Offset)); err != nil {
		return nil, err
	}
	return decodeBlockRecord(rec, hashLE)
}

// perFileLocatorNum marks locators that point at hash.bin (per-file layout).
const perFileLocatorNum = 0xffffffff

func (s *RawBlockStore) putPerFile(hashLE [32]byte, raw []byte) error {
	path := s.pathFor(hashLE)
	var data []byte
	var err error
	if s.opts.Zstd {
		data, err = encodeBlockRecord(hashLE, raw, true)
		if err != nil {
			return err
		}
	} else {
		data = raw
	}
	tmp := path + ".tmp"
	if !s.opts.Zstd && !writeBehindTestHooksActive() {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return err
		}
		// Keep a locator so IBD claim planning can skip already-stored heights without
		// Stat'ing every .bin (skipDisk used to re-getdata orphans and pack the window).
		loc := blockLocator{
			FileNum:      perFileLocatorNum,
			RecordLen:    uint32(len(data)),
			Uncompressed: uint32(len(raw)),
		}
		return writeBlockLocator(s.locatorRoot(), hashLE, loc)
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if stallAfterRawPutTmpWrite > 0 {
		time.Sleep(stallAfterRawPutTmpWrite)
	}
	if abortBeforeRawPutRename {
		abortBeforeRawPutRename = false
		return errAbortBeforeRawPutRename
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	loc := blockLocator{
		FileNum:      perFileLocatorNum,
		RecordLen:    uint32(len(data)),
		Uncompressed: uint32(len(raw)),
	}
	if s.opts.Zstd {
		loc.Flags = blockLocatorFlagZstd
	}
	return writeBlockLocator(s.locatorRoot(), hashLE, loc)
}

func (s *RawBlockStore) readPerFileLocator(hashLE [32]byte, loc blockLocator) ([]byte, error) {
	_ = loc
	path, ok := s.resolvePerFilePath(hashLE)
	if !ok {
		return nil, fmt.Errorf("block not in store")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if isPlainLegacyBin(b) {
		return b, nil
	}
	return decodeBlockRecord(b, hashLE)
}

func (s *RawBlockStore) getPerFile(hashLE [32]byte) ([]byte, error) {
	if loc, ok, err := readBlockLocator(s.locatorRoot(), hashLE); err == nil && ok && loc.FileNum == perFileLocatorNum {
		return s.readPerFileLocator(hashLE, loc)
	}
	path, ok := s.resolvePerFilePath(hashLE)
	if !ok {
		return nil, fmt.Errorf("block not in store")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if isPlainLegacyBin(b) {
		return b, nil
	}
	return decodeBlockRecord(b, hashLE)
}

// ProbeBundledContiguousTip scans bundled blk*.dat files in append order and returns the
// highest height present (-1 when empty). Used to reconcile rawblocks_sync.json with disk.
func (s *RawBlockStore) ProbeBundledContiguousTip() (int64, error) {
	if s.opts.Layout != BlockLayoutBundled {
		return -1, fmt.Errorf("bundled contiguous probe requires bundled layout")
	}
	s.mu.Lock()
	dir := s.dir
	s.mu.Unlock()
	files, err := listBundledBlkFiles(dir)
	if err != nil {
		return -1, err
	}
	var last int64 = -1
	var cur int64
	for _, fileNum := range files {
		path := bundledBlkPath(dir, fileNum)
		data, err := os.ReadFile(path)
		if err != nil {
			return last, err
		}
		off := 0
		for off+blockRecordHeaderLen <= len(data) {
			if binary.LittleEndian.Uint32(data[off:]) != blockRecordMagic {
				break
			}
			storedLen := binary.LittleEndian.Uint32(data[off+8 : off+12])
			recLen := blockRecordHeaderLen + int(storedLen)
			if recLen <= blockRecordHeaderLen || off+recLen > len(data) {
				break
			}
			last = cur
			cur++
			off += recLen
		}
	}
	if last < 0 {
		s.fileCount.Store(0)
	} else {
		s.fileCount.Store(last + 1)
	}
	return last, nil
}

// GetByContiguousHeight returns the raw block at the given height by scanning bundled
// blk*.dat files in append order (heights 0..contiguous_raw_height). This bypasses
// per-hash locators when bodies were stored sequentially but locator keys disagree
// with the header journal (operator recovery / partial re-sync).
func (s *RawBlockStore) GetByContiguousHeight(height int64) ([]byte, error) {
	if height < 0 {
		return nil, fmt.Errorf("negative contiguous height %d", height)
	}
	if s.opts.Layout != BlockLayoutBundled {
		return nil, fmt.Errorf("contiguous height read requires bundled layout")
	}
	s.mu.Lock()
	dir := s.dir
	s.mu.Unlock()
	files, err := listBundledBlkFiles(dir)
	if err != nil {
		return nil, err
	}
	var cur int64
	for _, fileNum := range files {
		path := bundledBlkPath(dir, fileNum)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		off := 0
		for off+blockRecordHeaderLen <= len(data) {
			if binary.LittleEndian.Uint32(data[off:]) != blockRecordMagic {
				break
			}
			storedLen := binary.LittleEndian.Uint32(data[off+8 : off+12])
			recLen := blockRecordHeaderLen + int(storedLen)
			if recLen <= blockRecordHeaderLen || off+recLen > len(data) {
				break
			}
			if cur == height {
				rec := data[off : off+recLen]
				var hash [32]byte
				copy(hash[:], rec[12:44])
				return decodeBlockRecord(rec, hash)
			}
			cur++
			off += recLen
		}
	}
	return nil, fmt.Errorf("contiguous height %d not in bundled store (tip %d)", height, cur-1)
}

func listBundledBlkFiles(rawDir string) ([]uint32, error) {
	var out []uint32
	for n := uint32(0); n < 100000; n++ {
		if _, err := os.Stat(bundledBlkPath(rawDir, n)); err != nil {
			if os.IsNotExist(err) {
				break
			}
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}
