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
	"sync"
	"sync/atomic"
	"time"

	"dogego/wire"
)

// RawBlockStore keeps full block message payloads on disk (per-file and/or bundled blk*.dat).
type RawBlockStore struct {
	dir        string
	opts       BlockStorageOpts
	mu         sync.Mutex
	txIndex    *TxIndex
	addrIndex  *AddrIndex
	indexingOn bool
	// deferIndexing, when set and true, skips tx/addr IndexBlock on Put (Core indexes on connect;
	// per-txid files during deep body IBD dominate disk I/O and starve block download).
	deferIndexing func() bool
	sideband   *BlockPutSideband
	readCache  *rawBlockReadCache
	compStats  *compressionStatsCache
	bytesDisk  *bytesOnDiskCache
	manifestOK bool
	// fileCount is -1 until first Count/FastCount refresh; then maintained on Put/Remove.
	fileCount atomic.Int64
}

// OpenRawBlockStore creates datadir/rawblocks with default per-file layout.
func OpenRawBlockStore(datadir string) (*RawBlockStore, error) {
	return OpenRawBlockStoreWithOpts(datadir, DefaultBlockStorageOpts())
}

// OpenRawBlockStoreWithOpts opens raw block storage using layout/zstd from config, reconciled
// with rawblocks/storage.json when the chain folder was already initialized.
func OpenRawBlockStoreWithOpts(datadir string, requested BlockStorageOpts) (*RawBlockStore, error) {
	d := filepath.Join(datadir, "rawblocks")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return nil, err
	}
	opts := ResolveBlockStorageOpts(requested, d)
	s := &RawBlockStore{
		dir:       d,
		opts:      opts,
		readCache: newRawBlockReadCache(rawBlockReadCacheMax),
	}
	s.fileCount.Store(-1)
	return s, nil
}

// StorageOpts returns the effective on-disk block storage options.
func (s *RawBlockStore) StorageOpts() BlockStorageOpts {
	if s == nil {
		return DefaultBlockStorageOpts()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opts
}

// SetBlockPutSideband configures post-store hooks (aux journal patch, mempool prune).
func (s *RawBlockStore) SetBlockPutSideband(b *BlockPutSideband) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sideband = b
}

// EnableTxIndexing registers a TxIndex to update whenever Put succeeds (full-node explorer mode).
func (s *RawBlockStore) EnableTxIndexing(ix *TxIndex, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.txIndex = ix
	s.indexingOn = on && ix != nil
}

// EnableAddrIndexing registers an AddrIndex to update whenever Put succeeds (explorer address search).
func (s *RawBlockStore) EnableAddrIndexing(ix *AddrIndex, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addrIndex = ix
	if on && ix != nil {
		// addr index requires tx index for cross-block prevout resolution during IBD.
	}
}

// SetDeferIndexing skips tx/addr IndexBlock on Put while fn returns true (deep body IBD).
// Call IndexStoredBlock after ConnectBlock so the index tracks chainActive instead.
func (s *RawBlockStore) SetDeferIndexing(fn func() bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deferIndexing = fn
}

// IndexStoredBlock updates tx/addr indexes for a stored block (ConnectBlock path; ignores defer).
func (s *RawBlockStore) IndexStoredBlock(hashLE [32]byte, raw []byte) {
	if s == nil || len(raw) < 80 {
		return
	}
	s.mu.Lock()
	ix := s.txIndex
	addrIx := s.addrIndex
	on := s.indexingOn
	s.mu.Unlock()
	if !on || ix == nil {
		return
	}
	if err := ix.IndexBlock(hashLE, raw); err != nil {
		fmt.Fprintf(os.Stderr, "tx index (connect): %v\n", err)
	}
	if addrIx != nil {
		if err := addrIx.IndexBlock(hashLE, raw); err != nil {
			fmt.Fprintf(os.Stderr, "addr index (connect): %v\n", err)
		}
	}
}

func (s *RawBlockStore) pathFor(hashLE [32]byte) string {
	name := hex.EncodeToString(hashLE[:])
	return filepath.Join(s.dir, name+".bin")
}

func (s *RawBlockStore) ensureManifestLocked() error {
	if s.manifestOK {
		return nil
	}
	if err := saveBlockStorageManifest(s.dir, s.opts); err != nil {
		return err
	}
	s.manifestOK = true
	return nil
}

// Has reports whether a block payload exists.
func (s *RawBlockStore) Has(hashLE [32]byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if loc, ok, err := readBlockLocator(s.locatorRoot(), hashLE); err == nil && ok {
		if loc.FileNum == perFileLocatorNum {
			_, err := os.Stat(s.pathFor(hashLE))
			return err == nil
		}
		_, err := os.Stat(bundledBlkPath(s.dir, loc.FileNum))
		return err == nil
	}
	_, err := os.Stat(s.pathFor(hashLE))
	return err == nil
}

// Put writes the raw block bytes (full serialized block as in the P2P "block" message body).
func (s *RawBlockStore) Put(hashLE [32]byte, raw []byte) error {
	if len(raw) < 80 {
		return fmt.Errorf("raw block too short %d", len(raw))
	}
	if err := wire.ValidateBlockPayload(raw, hashLE); err != nil {
		return fmt.Errorf("block validate: %w", err)
	}
	s.mu.Lock()
	had := s.hasLocked(hashLE)
	if err := s.ensureManifestLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	var putErr error
	switch s.opts.Layout {
	case BlockLayoutBundled:
		putErr = s.putBundled(hashLE, raw)
	default:
		putErr = s.putPerFile(hashLE, raw)
	}
	if putErr != nil {
		s.mu.Unlock()
		return putErr
	}
	ix := s.txIndex
	addrIx := s.addrIndex
	on := s.indexingOn
	deferIdx := s.deferIndexing
	sb := s.sideband
	s.mu.Unlock()
	if !had {
		s.bumpFileCount(1)
		s.InvalidateCompressionStatsCache()
		s.InvalidateBytesOnDiskCache()
	}
	skipIndex := deferIdx != nil && deferIdx()
	if on && ix != nil && !skipIndex {
		if err := ix.IndexBlock(hashLE, raw); err != nil {
			fmt.Fprintf(os.Stderr, "tx index (block stored): %v\n", err)
		}
	}
	if addrIx != nil && on && !skipIndex {
		if err := addrIx.IndexBlock(hashLE, raw); err != nil {
			fmt.Fprintf(os.Stderr, "addr index (block stored): %v\n", err)
		}
	}
	if sb != nil {
		sb.AfterPut(hashLE, raw)
	}
	if s.readCache != nil {
		s.readCache.put(hashLE, raw)
	}
	return nil
}

func (s *RawBlockStore) hasLocked(hashLE [32]byte) bool {
	if loc, ok, err := readBlockLocator(s.locatorRoot(), hashLE); err == nil && ok {
		if loc.FileNum == perFileLocatorNum {
			_, err := os.Stat(s.pathFor(hashLE))
			return err == nil
		}
		_, err := os.Stat(bundledBlkPath(s.dir, loc.FileNum))
		return err == nil
	}
	_, err := os.Stat(s.pathFor(hashLE))
	return err == nil
}

// Remove deletes a stored block (locator and optional per-file copy).
func (s *RawBlockStore) Remove(hashLE [32]byte) error {
	s.mu.Lock()
	had := s.hasLocked(hashLE)
	_ = removeBlockLocator(s.locatorRoot(), hashLE)
	path := s.pathFor(hashLE)
	s.mu.Unlock()
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if had {
		s.bumpFileCount(-1)
		s.InvalidateCompressionStatsCache()
		s.InvalidateBytesOnDiskCache()
	}
	if s.readCache != nil {
		s.readCache.drop(hashLE)
	}
	return nil
}

// Get returns the stored raw block payload for this block id (LE), or an error if missing.
func (s *RawBlockStore) Get(hashLE [32]byte) ([]byte, error) {
	if s.readCache != nil {
		if b, ok := s.readCache.get(hashLE); ok {
			return b, nil
		}
	}
	s.mu.Lock()
	b, err := s.getLocked(hashLE)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if s.readCache != nil {
		s.readCache.put(hashLE, b)
	}
	return b, nil
}

func (s *RawBlockStore) getLocked(hashLE [32]byte) ([]byte, error) {
	if loc, ok, err := readBlockLocator(s.locatorRoot(), hashLE); err == nil && ok {
		if loc.FileNum == perFileLocatorNum {
			return s.readPerFileLocator(hashLE, loc)
		}
		return s.getViaLocator(hashLE)
	}
	if s.opts.Layout == BlockLayoutBundled {
		return nil, fmt.Errorf("block not in store")
	}
	return s.getPerFile(hashLE)
}

// Dir returns the rawblocks directory path.
func (s *RawBlockStore) Dir() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dir
}

func (s *RawBlockStore) bumpFileCount(delta int64) {
	for {
		cur := s.fileCount.Load()
		if cur < 0 {
			return
		}
		next := cur + delta
		if next < 0 {
			next = 0
		}
		if s.fileCount.CompareAndSwap(cur, next) {
			return
		}
	}
}

func (s *RawBlockStore) scanFileCount() (int, error) {
	n, err := countBlockLocators(s.locatorRoot())
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	dir := s.dir
	s.mu.Unlock()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return n, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".bin" {
			continue
		}
		hexName := e.Name()[:len(e.Name())-4]
		b, err := hex.DecodeString(hexName)
		if err != nil || len(b) != 32 {
			continue
		}
		var hashLE [32]byte
		copy(hashLE[:], b)
		if _, ok, _ := readBlockLocator(s.locatorRoot(), hashLE); ok {
			continue
		}
		n++
	}
	return n, nil
}

// ReconcileCountCacheFromDisk refreshes the file counter from on-disk block locators and legacy *.bin.
func (s *RawBlockStore) ReconcileCountCacheFromDisk() {
	if s == nil {
		return
	}
	if s.StorageOpts().Layout == BlockLayoutBundled {
		tip, err := s.ProbeBundledContiguousTip()
		if err != nil {
			return
		}
		if tip < 0 {
			s.fileCount.Store(0)
			return
		}
		s.fileCount.Store(int64(tip) + 1)
		return
	}
	n, err := s.scanFileCount()
	if err != nil {
		return
	}
	s.fileCount.Store(int64(n))
}

// FastCount returns stored block count using a cached counter when available.
func (s *RawBlockStore) FastCount() (int, error) {
	if s == nil {
		return 0, nil
	}
	if n := s.fileCount.Load(); n >= 0 {
		return int(n), nil
	}
	if s.StorageOpts().Layout == BlockLayoutBundled {
		tip, err := s.ProbeBundledContiguousTip()
		if err != nil {
			return 0, err
		}
		if tip < 0 {
			return 0, nil
		}
		return int(tip) + 1, nil
	}
	n, err := s.scanFileCount()
	if err != nil {
		return 0, err
	}
	s.fileCount.Store(int64(n))
	return n, nil
}

// Count returns how many blocks are stored (uses FastCount).
func (s *RawBlockStore) Count() (int, error) {
	return s.FastCount()
}

// BytesOnDisk sums rawblocks payload bytes (blk*.dat, *.bin, excluding tiny index files).
func (s *RawBlockStore) BytesOnDisk() (int64, error) {
	s.mu.Lock()
	root := s.dir
	s.mu.Unlock()
	var total int64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "loc" {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if filepath.Ext(name) == ".bin" || (len(name) >= 7 && name[:3] == "blk" && filepath.Ext(name) == ".dat") {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

type bytesOnDiskCache struct {
	mu      sync.Mutex
	scanned time.Time
	ttl     time.Duration
	bytes   int64
	err     error
}

// CachedBytesOnDisk returns BytesOnDisk results, refreshing when older than ttl.
// getblockchaininfo uses this to avoid a full rawblocks tree walk on every RPC during IBD.
func (s *RawBlockStore) CachedBytesOnDisk(ttl time.Duration) (int64, error) {
	if s == nil {
		return 0, nil
	}
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	s.mu.Lock()
	cache := s.bytesDisk
	if cache == nil {
		cache = &bytesOnDiskCache{ttl: ttl}
		s.bytesDisk = cache
	}
	s.mu.Unlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()
	if time.Since(cache.scanned) < cache.ttl && cache.scanned.After(time.Time{}) {
		return cache.bytes, cache.err
	}
	b, err := s.BytesOnDisk()
	cache.bytes = b
	cache.err = err
	cache.scanned = time.Now()
	return b, err
}

// InvalidateBytesOnDiskCache clears cached on-disk byte totals (after bulk import).
func (s *RawBlockStore) InvalidateBytesOnDiskCache() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cache := s.bytesDisk
	s.mu.Unlock()
	if cache == nil {
		return
	}
	cache.mu.Lock()
	cache.scanned = time.Time{}
	cache.mu.Unlock()
}
