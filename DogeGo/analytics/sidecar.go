// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// JournalTip is the header store surface needed by the embedded sidecar.
type JournalTip interface {
	TipHeight() (int64, error)
}

// RawBlockBinCounter counts *.bin payloads under rawblocks/ (full node only).
type RawBlockBinCounter interface {
	Count() (int, error)
}

// SidecarConfig configures the background indexer that keeps dogego_analytics.db
// aligned with local headers and (when set) raw block file counts - Core-style
// auxiliary catalog, not Dogecoin Core chainstate LevelDB.
type SidecarConfig struct {
	ChainRoot      string
	NetworkSlug    string
	GenesisHashHex string
	Journal        JournalTip
	RawBlocks      RawBlockBinCounter
	// DB optional shared read-write handle (when set, RunSidecar does not open/close its own DB).
	DB *DB
	// SampleMetrics, when set, records mempool / block / disk sizes into metric_samples each tick.
	SampleMetrics func() LiveMetrics
	Log           func(tag, line string)
	Tick          time.Duration
}

// RunSidecar opens the analytics DB under ChainRoot, writes meta, then until ctx
// is cancelled periodically updates index_progress (headers tip; raw *.bin count
// when RawBlocks is set). Safe for SPV (RawBlocks nil): headers + meta only.
func RunSidecar(ctx context.Context, cfg SidecarConfig) {
	log := cfg.Log
	if log == nil {
		log = func(_, _ string) {}
	}
	tick := cfg.Tick
	if tick < 5*time.Second {
		tick = 25 * time.Second
	}
	dbPath := filepath.Join(cfg.ChainRoot, "dogego_analytics.db")
	db := cfg.DB
	ownDB := db == nil
	if ownDB {
		var err error
		db, err = Open(dbPath)
		if err != nil {
			log("indexer", fmt.Sprintf("analytics open %s: %v", dbPath, err))
			return
		}
	}
	if ownDB {
		defer db.Close()
	}

	_ = SetMeta(db, "chain_root", cfg.ChainRoot)
	_ = SetMeta(db, "network", cfg.NetworkSlug)
	_ = SetMeta(db, "genesis_hash_hex", cfg.GenesisHashHex)
	_ = SetMeta(db, "embedded_sidecar", "1")
	_ = SetMeta(db, "core_note", "Aux Pebble catalog (headers/raw bin snapshots); not Core blocks/chainstate.")

	extra := "; SPV: no raw block store"
	if cfg.RawBlocks != nil {
		extra = "; full node: also raw block bin count"
	}
	log("indexer", "embedded analytics indexer running (Pebble sidecar; updates headers tip on an interval"+extra+")")

	sync := func() {
		if cfg.Journal == nil {
			return
		}
		tip, err := cfg.Journal.TipHeight()
		if err != nil {
			log("indexer", "header tip: "+err.Error())
			return
		}
		if err := RecordHeadersSynced(db, tip); err != nil {
			log("indexer", "headers index_progress: "+err.Error())
		}
		if cfg.RawBlocks != nil {
			n, err := cfg.RawBlocks.Count()
			if err != nil {
				log("indexer", "rawblocks count: "+err.Error())
				return
			}
			if err := RecordRawBlockScan(db, n); err != nil {
				log("indexer", "rawblocks index_progress: "+err.Error())
				return
			}
			_ = SetMeta(db, "rawblocks_bin_count", fmt.Sprint(n))
		}
		if cfg.SampleMetrics != nil {
			if err := RecordMetricSample(db, cfg.SampleMetrics()); err != nil {
				log("indexer", "metric sample: "+err.Error())
			}
		}
	}
	sync()

	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			log("indexer", "embedded analytics indexer stopped")
			return
		case <-t.C:
			sync()
		}
	}
}
