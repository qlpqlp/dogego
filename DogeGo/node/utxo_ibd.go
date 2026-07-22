// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strconv"
	"sync"

	"dogego/applog"
	"dogego/store"
)

const utxoSnapshotBlockInterval = 10_000

// utxoIBDSync extends the UTXO cache during forward block download (Core connects blocks incrementally).
type utxoIBDSync struct {
	mu              sync.Mutex
	lastSyncedCont  int64
	lastSnapshotCont int64
	chainRoot       string
}

func newUtxoIBDSync(chainRoot string) *utxoIBDSync {
	return &utxoIBDSync{chainRoot: chainRoot, lastSyncedCont: -1}
}

func (u *utxoIBDSync) noteContiguous(cont int64) {
	if u == nil || cont < 0 {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if cont > u.lastSyncedCont {
		u.lastSyncedCont = cont
	}
}

func (u *utxoIBDSync) onContiguousAdvance(bs *BlockStoreCtx, utxo *store.UtxoCache) {
	if u == nil || bs == nil || utxo == nil {
		return
	}
	cont := bs.ContiguousRawHeight()
	if cont < 0 {
		return
	}
	// Connect catch-up worker owns UTXO replay while stored bodies run far ahead of chainActive.
	if ConnectCatchUpLag(bs, utxo) >= EffectiveConnectCatchUpMinLag(bs) {
		u.noteContiguous(cont)
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	interval := int64(utxoSyncIntervalNormal)
	if BodiesBehindHeaders(bs) {
		interval = int64(utxoSyncIntervalBulkIBD)
		// Early mainnet IBD: connect more often so chainActive and verification_progress track bodies.
		if cont < 50_000 {
			interval = 128
		}
		if utxoTip := utxo.TipHeight(); utxoTip >= 0 {
			backlog := cont - utxoTip
			if backlog > 4096 {
				interval = 32
			} else if backlog > 1024 {
				interval = 64
			}
		}
	}
	if u.lastSyncedCont >= 0 && cont-u.lastSyncedCont < interval {
		return
	}
	if err := bs.SyncUtxoCache(); err != nil {
		applog.Line("utxo", "IBD sync: "+err.Error())
		return
	}
	u.lastSyncedCont = cont
	if utxo.TipHeight() >= 0 {
		applog.Line("utxo", "UTXO cache advanced through height "+strconv.FormatInt(utxo.TipHeight(), 10))
	}
	if u.chainRoot != "" && cont >= 0 && BodiesAlignedForUtxoSnapshot(bs, utxo) &&
		(u.lastSnapshotCont < 0 || cont-u.lastSnapshotCont >= utxoSnapshotBlockInterval) {
		if started, err := utxo.SaveSnapshotAsync(store.UtxoSnapshotPath(u.chainRoot)); err != nil {
			applog.Line("utxo", "IBD snapshot save: "+err.Error())
		} else if started {
			u.lastSnapshotCont = cont
		}
	}
}
