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
	"sort"
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
	loc, err := s.appendBundledRecord(rec, uint32(len(raw)))
	if err != nil {
		return err
	}
	return writeBlockLocator(s.locatorRoot(), hashLE, loc)
}

func (s *RawBlockStore) appendBundledRecord(rec []byte, uncompressed uint32) (blockLocator, error) {
	fileNum, offset, err := s.pickBundledAppendSlot(int64(len(rec)))
	if err != nil {
		return blockLocator{}, err
	}
	path := bundledBlkPath(s.dir, fileNum)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return blockLocator{}, err
	}
	if _, err := f.Seek(offset, 0); err != nil {
		_ = f.Close()
		return blockLocator{}, err
	}
	if _, err := f.Write(rec); err != nil {
		_ = f.Close()
		return blockLocator{}, err
	}
	// Do not fsync every block (Core buffers blk*.dat writes). Sync only when rotating
	// to a new file so IBD is not disk-bound on slow HDDs/Windows.
	if offset == 0 {
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return blockLocator{}, err
		}
	}
	_ = f.Close()
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

func (s *RawBlockStore) pickBundledAppendSlot(need int64) (fileNum uint32, offset int64, err error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, 0, err
	}
	var tipNum uint32
	var tipSize int64 = -1
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) != 12 || name[:3] != "blk" || filepath.Ext(name) != ".dat" {
			continue
		}
		var n uint32
		if _, err := fmt.Sscanf(name, "blk%05d.dat", &n); err != nil {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if n >= tipNum {
			tipNum = n
			tipSize = fi.Size()
		}
	}
	if tipSize >= 0 {
		if fi, err := os.Stat(bundledBlkPath(s.dir, tipNum)); err == nil {
			tipSize = fi.Size()
		}
	}
	if tipSize < 0 {
		return 0, 0, nil
	}
	if tipSize+need <= maxBundledFileBytes {
		return tipNum, tipSize, nil
	}
	return tipNum + 1, 0, nil
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
		return removeBlockLocator(s.locatorRoot(), hashLE)
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
	if s.opts.Zstd {
		loc := blockLocator{
			FileNum:      perFileLocatorNum,
			RecordLen:    uint32(len(data)),
			Uncompressed: uint32(len(raw)),
			Flags:        blockLocatorFlagZstd,
		}
		return writeBlockLocator(s.locatorRoot(), hashLE, loc)
	}
	return removeBlockLocator(s.locatorRoot(), hashLE)
}

func (s *RawBlockStore) readPerFileLocator(hashLE [32]byte, loc blockLocator) ([]byte, error) {
	b, err := os.ReadFile(s.pathFor(hashLE))
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
	b, err := os.ReadFile(s.pathFor(hashLE))
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
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return nil, err
	}
	var out []uint32
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) != 12 || name[:3] != "blk" || filepath.Ext(name) != ".dat" {
			continue
		}
		var n uint32
		if _, err := fmt.Sscanf(name, "blk%05d.dat", &n); err != nil {
			continue
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}
