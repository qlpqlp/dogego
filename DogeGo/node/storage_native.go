// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"path/filepath"
	"sync"
	"time"

	"dogego/analytics"
	"dogego/store"
)

type storageSummaryCache struct {
	mu      sync.Mutex
	scanned time.Time
	ttl     time.Duration
	summary map[string]interface{}
}

func (c *storageSummaryCache) get(ttl time.Duration, build func() map[string]interface{}) map[string]interface{} {
	if c == nil {
		return build()
	}
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.summary != nil && time.Since(c.scanned) < c.ttl && c.scanned.After(time.Time{}) {
		out := make(map[string]interface{}, len(c.summary))
		for k, v := range c.summary {
			out[k] = v
		}
		return out
	}
	sm := build()
	c.summary = sm
	c.scanned = time.Now()
	c.ttl = ttl
	out := make(map[string]interface{}, len(sm))
	for k, v := range sm {
		out[k] = v
	}
	return out
}

func nativeStorageSummary(chainRoot string, rb *store.RawBlockStore, txIx *store.TxIndex, contiguousBody func() int64) func() map[string]interface{} {
	var cache storageSummaryCache
	return func() map[string]interface{} {
		// Heavy dir walks (txindex file counts, full rawblocks *.bin) compete with body IBD I/O.
		// Keep results longer so /api/summary does not re-walk millions of files every minute.
		return cache.get(5*time.Minute, func() map[string]interface{} {
			sm := map[string]interface{}{
				"storage_mode": store.StorageNative,
				"layout":       "native",
			}
			cont := int64(-1)
			if contiguousBody != nil {
				cont = contiguousBody()
			}
			deepBodyIBD := cont >= 0 && cont < 5_050_000
			var headersBytes, utxoBytes, rawB, txB int64
			if chainRoot != "" {
				sm["native_headers"] = filepath.Join(chainRoot, "headers.bin")
				headersBytes += analytics.SubdirSizeIfExists(filepath.Join(chainRoot, "headers"))
				headersBytes += analytics.SubdirSizeIfExists(filepath.Join(chainRoot, "headers.bin"))
				headersBytes += analytics.SubdirSizeIfExists(filepath.Join(chainRoot, "headers_aux.bin"))
				sm["headers_bytes"] = headersBytes
				utxoBytes = analytics.SubdirSizeIfExists(filepath.Join(chainRoot, "utxo.cache"))
				if utxoBytes > 0 {
					sm["utxo_bytes"] = utxoBytes
				}
				if !deepBodyIBD {
					_, fullRaw, fullTx, _ := analytics.ChainStoreBytes(chainRoot)
					rawB = fullRaw
					txB = fullTx
					sm["rawblocks_bytes"] = rawB
					sm["txindex_bytes"] = txB
				}
			}
			approx := deepBodyIBD
			if rb != nil {
				sm["native_rawblocks"] = rb.Dir()
				opts := rb.StorageOpts()
				sm["block_layout"] = opts.Layout
				sm["block_zstd"] = opts.Zstd
				if !deepBodyIBD {
					if n, err := rb.Count(); err == nil {
						sm["native_raw_block_count"] = n
					}
				} else if cont >= 0 {
					sm["native_raw_block_count"] = int(cont) + 1
				}
				// Cheap path during IBD: blk*.dat + cached leftover *.bin (no UI-thread walk).
				if cheap, err := rb.CachedBytesOnDisk(5 * time.Minute); err == nil {
					rawB = cheap
					sm["rawblocks_bytes"] = rawB
					if !rb.PayloadBytesReady() {
						approx = true
					}
				}
				if !deepBodyIBD {
					if cs, err := rb.CachedCompressionStats(45 * time.Second); err == nil && cs.BlockCount > 0 {
						sm["blocks_logical_bytes"] = cs.LogicalBytes
						sm["blocks_stored_payload_bytes"] = cs.StoredPayloadBytes
						sm["compression_ratio"] = cs.CompressionRatio
						sm["compression_savings_pct"] = cs.CompressionSavingsPct
					}
				}
			}
			// Chain disk = occupied chain data (headers + bodies + utxo [+ txindex when scanned]).
			sm["chain_bytes_total"] = headersBytes + rawB + utxoBytes + txB
			if approx {
				sm["disk_bytes_approx"] = true
			}
			sm["native_tx_index"] = txIx != nil
			if txIx != nil && !deepBodyIBD {
				if legacy, v2, err := txIx.FormatStats(); err == nil {
					sm["native_txindex_legacy_files"] = legacy
					sm["native_txindex_v2_files"] = v2
				}
			}
			if cont >= 0 {
				sm["native_contiguous_body_height"] = cont
			}
			if free, total, err := analytics.VolumeUsage(chainRoot); err == nil && total > 0 {
				sm["volume_free_bytes"] = int64(free)
				sm["volume_total_bytes"] = int64(total)
				usedPct := 100.0 * float64(total-free) / float64(total)
				sm["volume_used_percent"] = usedPct
			}
			return sm
		})
	}
}
