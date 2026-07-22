// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"dogego/store"
)

// journalTipForDashboard returns header tip/count for /api/summary without blocking on header sync locks.
func journalTipForDashboard(j *store.HeaderJournal) (tip, count int64, err error) {
	if j == nil {
		return -1, 0, nil
	}
	if j.HeaderLayout() == "segments" {
		if m, ok := store.ReadSegmentManifest(j.ChainDir()); ok {
			return m.TipHeight, m.TipHeight + 1, nil
		}
	}
	tip, count, err = j.SyncTipFromDisk()
	if err != nil {
		return tip, count, err
	}
	if mtip, ok := store.ReadSegmentManifestTip(j.ChainDir()); ok && mtip > tip {
		tip = mtip
		count = mtip + 1
	}
	return tip, count, nil
}

// journalBestHashForDashboard returns tip block hash without waiting on in-memory journal locks.
func journalBestHashForDashboard(j *store.HeaderJournal) (string, error) {
	if j == nil {
		return "", nil
	}
	if j.HeaderLayout() == "segments" {
		if m, ok := store.ReadSegmentManifest(j.ChainDir()); ok && m.TipHashHex != "" {
			return m.TipHashHex, nil
		}
	}
	return j.BestBlockHashHex()
}
