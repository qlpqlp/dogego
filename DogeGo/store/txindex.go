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
	"sync"
	"time"

	"dogego/wire"
)

// TxIndex maps confirmed transaction ids (RPC display hex) to block location and optional
// serialized tx bytes (v2 entries). Written when raw blocks are stored and indexing is enabled.
// This is not Core's full -txindex LevelDB (no address/UTXO sweep); it covers blocks under rawblocks/.
type TxIndex struct {
	root string
	mu   sync.Mutex
	// EmbedTx when true writes v2 entries (block hash + tx index + serialized tx).
	// When false, only 36-byte pointers are stored; txs load from raw block store (smaller disk).
	EmbedTx bool

	statsCache *txIndexStatsCache
}

// RootDir returns the tx index directory path.
func (x *TxIndex) RootDir() string {
	if x == nil {
		return ""
	}
	return x.root
}

// OpenTxIndex creates <chainDataDir>/indexes/tx for per-txid metadata files.
func OpenTxIndex(chainDataDir string) (*TxIndex, error) {
	return OpenTxIndexWithOpts(chainDataDir, true)
}

// OpenTxIndexWithOpts opens indexes/tx; embedTx controls v2 embedded tx bytes vs offset-only entries.
func OpenTxIndexWithOpts(chainDataDir string, embedTx bool) (*TxIndex, error) {
	d := filepath.Join(chainDataDir, "indexes", "tx")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return nil, err
	}
	return &TxIndex{root: d, EmbedTx: embedTx}, nil
}

// TxIDIndexFileNameLE returns the indexes/tx filename for a wire-order tx hash (tests / operators).
func TxIDIndexFileNameLE(h [32]byte) string {
	return txidRPCFileName(h)
}

func txidRPCFileName(h [32]byte) string {
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		b[i] = h[31-i]
	}
	return strings.ToLower(hex.EncodeToString(b))
}

// IndexBlock records each tx in this serialized block payload.
func (x *TxIndex) IndexBlock(blockHashLE [32]byte, raw []byte) error {
	if x == nil {
		return nil
	}
	dirMu := txIndexDirLock(x.root)
	return wire.ForEachBlockTx(raw, func(i uint32, tx *wire.Tx) error {
		id := txidRPCFileName(tx.TxHash())
		ser, err := tx.Serialize()
		if err != nil {
			return fmt.Errorf("tx index serialize: %w", err)
		}
		if len(ser) > 4_000_000 {
			return fmt.Errorf("tx index: tx too large %d bytes", len(ser))
		}
		var txRaw []byte
		if x.EmbedTx {
			txRaw = ser
		}
		entry := encodeTxIndexEntry(blockHashLE, i, txRaw)
		path := filepath.Join(x.root, id)
		// Shared dir lock: live IndexBlock + RepairTxIndex* open separate *TxIndex values;
		// without this Windows fails rename with "being used by another process".
		dirMu.Lock()
		x.mu.Lock()
		err = atomicWriteFileStall(path, entry, 0o600, stallAfterTxIndexTmpWrite)
		x.mu.Unlock()
		dirMu.Unlock()
		if err != nil {
			return fmt.Errorf("tx index %s: %w", id[:16], err)
		}
		x.invalidateStatsCache()
		return nil
	})
}

// Lookup returns block hash (LE) and tx index for a confirmed txid (64 hex, any case).
func (x *TxIndex) Lookup(txidHex string) (blockHashLE [32]byte, txIndex uint32, err error) {
	hit, err := x.LookupHit(txidHex)
	if err != nil {
		return [32]byte{}, 0, err
	}
	return hit.BlockHashLE, hit.TxIndex, nil
}

// LookupHit returns index metadata and optional serialized tx (v2 entries).
func (x *TxIndex) LookupHit(txidHex string) (TxIndexHit, error) {
	var empty TxIndexHit
	if x == nil {
		return empty, fmt.Errorf("tx index disabled")
	}
	txidHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(txidHex), "0x"))
	if len(txidHex) != 64 {
		return empty, fmt.Errorf("txid must be 64 hex characters")
	}
	for _, c := range txidHex {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return empty, fmt.Errorf("txid must be hex")
	}
	path := filepath.Join(x.root, txidHex)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty, fmt.Errorf("transaction not in local index (need the block stored under rawblocks/)")
		}
		return empty, err
	}
	return decodeTxIndexEntry(data)
}

// RemoveBlock deletes tx index entries that point at blockHashLE.
func (x *TxIndex) RemoveBlock(blockHashLE [32]byte) error {
	if x == nil {
		return nil
	}
	x.mu.Lock()
	root := x.root
	x.mu.Unlock()
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) != 64 {
			continue
		}
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(data) < txIndexMetaLen {
			continue
		}
		var got [32]byte
		copy(got[:], data[:32])
		if got != blockHashLE {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Stats counts txid metadata files (64 lowercase hex filenames) and sums their sizes.
func (x *TxIndex) Stats() (files int, totalBytes int64, err error) {
	return x.CachedStats(0)
}

type txIndexStatsCache struct {
	mu      sync.Mutex
	scanned time.Time
	ttl     time.Duration
	files   int
	bytes   int64
	err     error
}

func (x *TxIndex) invalidateStatsCache() {
	if x == nil || x.statsCache == nil {
		return
	}
	x.statsCache.mu.Lock()
	x.statsCache.scanned = time.Time{}
	x.statsCache.mu.Unlock()
}

// CachedStats returns Stats results, refreshing when older than ttl (default 60s when ttl <= 0).
// getblockchaininfo uses this to avoid scanning millions of tx index files on every RPC poll.
func (x *TxIndex) CachedStats(ttl time.Duration) (files int, totalBytes int64, err error) {
	if x == nil {
		return 0, 0, fmt.Errorf("nil tx index")
	}
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	if x.statsCache == nil {
		x.statsCache = &txIndexStatsCache{ttl: ttl}
	}
	c := x.statsCache
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.scanned) < c.ttl && c.scanned.After(time.Time{}) {
		return c.files, c.bytes, c.err
	}
	files, totalBytes, err = x.statsScanLocked()
	c.files = files
	c.bytes = totalBytes
	c.err = err
	c.scanned = time.Now()
	return files, totalBytes, err
}

func (x *TxIndex) statsScanLocked() (files int, totalBytes int64, err error) {
	if x == nil {
		return 0, 0, fmt.Errorf("nil tx index")
	}
	x.mu.Lock()
	root := x.root
	x.mu.Unlock()
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) != 64 {
			continue
		}
		valid := true
		for _, c := range name {
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
				continue
			}
			valid = false
			break
		}
		if !valid {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		files++
		totalBytes += fi.Size()
	}
	return files, totalBytes, nil
}
