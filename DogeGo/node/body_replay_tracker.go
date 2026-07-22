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

// bodyReplayTracker detects the transition from UTXO-snapshot-ahead body replay to aligned
// bodies and triggers a safe checkpoint (better-than-Core: automatic aligned utxo.cache).
type bodyReplayTracker struct {
	mu        sync.Mutex
	wasAhead  bool
	completed bool
}

func (t *bodyReplayTracker) seedWasAhead() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.wasAhead = true
	t.mu.Unlock()
}

func (t *bodyReplayTracker) noteContiguousAdvance(bs *BlockStoreCtx, utxo *store.UtxoCache, cont int64, chainRoot string, syncCheckpoint func(int64), onComplete func(*BlockStoreCtx, *store.UtxoCache, int64)) {
	if t == nil || bs == nil || utxo == nil || cont < 0 {
		return
	}
	ahead := bs.utxoAheadOfStoredBodies()
	t.mu.Lock()
	if ahead {
		t.wasAhead = true
		t.mu.Unlock()
		return
	}
	if t.completed || !t.wasAhead {
		t.mu.Unlock()
		return
	}
	t.completed = true
	t.mu.Unlock()

	applog.Line("block", "UTXO snapshot body replay complete at height "+strconv.FormatInt(cont, 10)+"; resuming connect catch-up toward header tip")
	if syncCheckpoint != nil {
		syncCheckpoint(cont)
	}
	if chainRoot != "" {
		path := store.UtxoSnapshotPath(chainRoot)
		go func() {
			if err := PersistUtxoSnapshotIfAligned(bs, utxo, path, "replay_complete"); err != nil {
				applog.Line("utxo", "replay-complete snapshot: "+err.Error())
			}
		}()
	}
	if onComplete != nil {
		onComplete(bs, utxo, cont)
	}
}

// onBodyReplayComplete resumes connect catch-up after UTXO snapshot body replay aligns stored bodies.
func onBodyReplayComplete(bs *BlockStoreCtx, utxo *store.UtxoCache, cont int64, rawFill *progressiveRawState) {
	if bs == nil || utxo == nil || cont < 0 {
		return
	}
	if rawFill != nil {
		rawFill.ResumeAfterSnapshotReplay(bs)
	}
	bs.FlushDeferredConnect()
	go func() {
		if err := bs.SyncUtxoCacheBounded(4096); err != nil {
			applog.Line("utxo", "post-replay connect kick: "+err.Error())
		}
	}()
}
