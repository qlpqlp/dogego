// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"sync"
	"time"

	"dogego/store"
)

const rawCountReconcileMinInterval = 60 * time.Second

var (
	rawCountReconcileMu sync.Mutex
	rawCountReconcileAt time.Time
)

// maybeReconcileRawBlockCount refreshes the raw block file counter at most once per minute during IBD.
func maybeReconcileRawBlockCount(raw *store.RawBlockStore, contiguousH int64, cached int) int {
	if raw == nil || contiguousH < 0 || int64(cached) >= contiguousH {
		return cached
	}
	// Bundled FastCount used to ReadFile every blk*.dat; never rescan from the dashboard path.
	if raw.StorageOpts().Layout == store.BlockLayoutBundled {
		if cached > 0 {
			return cached
		}
		return int(contiguousH) + 1
	}
	rawCountReconcileMu.Lock()
	now := time.Now()
	if now.Sub(rawCountReconcileAt) < rawCountReconcileMinInterval {
		rawCountReconcileMu.Unlock()
		if cached > 0 {
			return cached
		}
		return int(contiguousH) + 1
	}
	rawCountReconcileAt = now
	rawCountReconcileMu.Unlock()
	raw.ReconcileCountCacheFromDisk()
	if n, err := raw.FastCount(); err == nil && int64(n) >= contiguousH {
		return n
	}
	if cached > 0 {
		return cached
	}
	return int(contiguousH) + 1
}
