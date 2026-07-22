// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"path/filepath"
	"strconv"

	"dogego/applog"
	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// utxoSnapshotBodyMargin allows a small connect lag between stored bodies and UTXO tip.
const utxoSnapshotBodyMargin = 32

// utxoStartupConnectNeeded reports whether startup should run SyncUtxoCache after loading utxo.cache.
func utxoStartupConnectNeeded(bs *BlockStoreCtx, utxo *store.UtxoCache) bool {
	if utxo == nil || utxo.TipHeight() < 0 {
		return true
	}
	if bs == nil {
		return false
	}
	cont := bs.ContiguousRawHeight()
	if cont < 0 {
		return true
	}
	return utxo.TipHeight() < cont
}

// BodiesAlignedForUtxoSnapshot reports whether on-disk raw bodies cover the UTXO tip
// closely enough to persist utxo.cache (prevents snapshot-ahead-of-bodies restart loops).
func BodiesAlignedForUtxoSnapshot(bs *BlockStoreCtx, utxo *store.UtxoCache) bool {
	if utxo == nil || utxo.TipHeight() < 0 {
		return false
	}
	if bs == nil {
		return true
	}
	cont := bs.ContiguousRawHeight()
	if cont < 0 {
		return false
	}
	return utxo.TipHeight() <= cont+utxoSnapshotBodyMargin
}

// PersistUtxoSnapshotIfAligned writes utxo.cache only when stored bodies cover the UTXO tip.
func PersistUtxoSnapshotIfAligned(bs *BlockStoreCtx, utxo *store.UtxoCache, path string, reason string) error {
	if utxo == nil || path == "" || utxo.TipHeight() < 0 {
		return nil
	}
	if !BodiesAlignedForUtxoSnapshot(bs, utxo) {
		cont := int64(-1)
		if bs != nil {
			cont = bs.ContiguousRawHeight()
		}
		applog.Line("utxo", "snapshot save skipped ("+reason+"): bodies through "+strconv.FormatInt(cont, 10)+
			" lag UTXO tip "+strconv.FormatInt(utxo.TipHeight(), 10))
		return nil
	}
	if err := utxo.SaveSnapshot(path); err != nil {
		return err
	}
	applog.Line("utxo", "saved utxo snapshot through height "+strconv.FormatInt(utxo.TipHeight(), 10)+" ("+reason+")")
	if bs != nil {
		chainRoot := filepath.Dir(path)
		tipHash := ""
		if bs.Journal != nil {
			if h80, err := bs.Journal.ReadHeaderAt(utxo.TipHeight()); err == nil && len(h80) >= 80 {
				tipHash = pow.BlockHashHex(h80)
			}
		}
		_ = store.SaveChainActiveManifest(chainRoot, store.ChainActiveManifest{
			UtxoTipHeight:       utxo.TipHeight(),
			UtxoTipBlockHash:    tipHash,
			ContiguousRawHeight: bs.ContiguousRawHeight(),
		})
	}
	return nil
}

// shouldPersistSyncCheckpoint returns true when rawblocks_sync.json should be flushed.
func shouldPersistSyncCheckpoint(cont int64, bs *BlockStoreCtx) bool {
	if cont < 0 {
		return false
	}
	if cont < 128 {
		return true
	}
	if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) && cont < 50_000 {
		return cont%16 == 0
	}
	if bs != nil && bs.utxoAheadOfStoredBodies() {
		if bs.Utxo != nil && bs.Utxo.TipHeight() >= 0 {
			if remain := bs.Utxo.TipHeight() - cont; remain > 0 && remain <= 64 {
				return true
			}
		}
		return cont%16 == 0
	}
	return cont%64 == 0
}

// ShouldPreserveContiguousCache reports when header recovery must not wipe monotonic body
// coverage during forward block IBD or UTXO-snapshot body replay.
func ShouldPreserveContiguousCache(bs *BlockStoreCtx) bool {
	if bs == nil {
		return false
	}
	if bs.utxoAheadOfStoredBodies() {
		return true
	}
	if BodiesBehindHeaders(bs) && bs.ContiguousRawHeight() >= 512 {
		return true
	}
	return false
}

// MaybeResetContiguousAfterHeaderRewind refreshes or resets cached body coverage after a header
// journal rewind. During forward body IBD, never drops cached progress to a genesis scan.
func MaybeResetContiguousAfterHeaderRewind(bs *BlockStoreCtx) {
	if bs == nil {
		return
	}
	if ShouldPreserveContiguousCache(bs) {
		return
	}
	bs.ResetContiguousTip()
}

// UtxoReplayTargetHeight is the body replay finish line: in-memory UTXO tip when set, else disk meta.
func UtxoReplayTargetHeight(bs *BlockStoreCtx, utxo *store.UtxoCache, diskTip int64) int64 {
	if utxo != nil && utxo.TipHeight() >= 0 {
		return utxo.TipHeight()
	}
	if diskTip >= 0 {
		if bs != nil {
			cont := bs.ContiguousRawHeight()
			if cont >= 0 && diskTip > cont+utxoSnapshotBodyMargin {
				return cont
			}
		}
		return diskTip
	}
	if bs != nil {
		return bs.ContiguousRawHeight()
	}
	return -1
}

// LoadUtxoSnapshotAtStartup loads utxo.cache. Legitimate UTXO-ahead-of-bodies replay snapshots
// are kept (better-than-Core fast IBD). Only fabricated saves (high tip, no coins, no bodies) are quarantined.
func LoadUtxoSnapshotAtStartup(path, chainDir string, j *store.HeaderJournal, raw *store.RawBlockStore, net chain.Network) (*store.UtxoCache, bool, error) {
	if chainDir != "" {
		if tip, from, err := store.TryRestoreBestQuarantinedUtxoSnapshot(chainDir); err != nil {
			applog.Line("utxo", "stale snapshot restore: "+err.Error())
		} else if tip >= 0 && from != "" {
			applog.Line("utxo", fmt.Sprintf("restored quarantined utxo.cache through height %d from %s", tip, from))
		}
		if n, err := store.PurgeStaleUtxoSnapshotTemps(chainDir); err != nil {
			applog.Line("utxo", "utxo.cache.tmp purge: "+err.Error())
		} else if n > 0 {
			applog.Line("utxo", "removed stale utxo.cache.tmp")
		}
	}
	cp, err := store.LoadRawBlockSyncCheckpoint(chainDir)
	if err != nil {
		return nil, false, err
	}
	diskTip, _, metaErr := store.ReadUtxoSnapshotDiskMeta(path)
	if metaErr != nil {
		if qerr := store.QuarantineUtxoSnapshot(path, "corrupt"); qerr != nil {
			applog.Line("utxo", "quarantine corrupt utxo.cache meta: "+qerr.Error())
		} else {
			applog.Line("utxo", "quarantined unreadable utxo.cache ("+metaErr.Error()+")")
		}
		return store.NewUtxoCache(), true, nil
	}
	if diskTip >= 0 && shouldQuarantineFabricatedUtxoSnapshot(diskTip, path, cp.ContiguousRawHeight, j, raw, net) {
		if err := store.QuarantineUtxoSnapshot(path, "bodies_missing"); err != nil {
			applog.Line("utxo", "quarantine misaligned utxo.cache: "+err.Error())
		} else {
			measured := measureStartupBodyContiguous(j, raw, net, cp.ContiguousRawHeight)
			applog.Line("utxo", fmt.Sprintf("quarantined fabricated utxo.cache (disk tip %d, bodies through %d, checkpoint %d)",
				diskTip, measured, cp.ContiguousRawHeight))
			return store.NewUtxoCache(), true, nil
		}
	}
	loaded, err := store.LoadUtxoSnapshot(path)
	if err != nil {
		if qerr := store.QuarantineUtxoSnapshot(path, "corrupt"); qerr != nil {
			applog.Line("utxo", "quarantine corrupt utxo.cache: "+qerr.Error())
			return nil, false, err
		}
		applog.Line("utxo", "quarantined corrupt utxo.cache ("+err.Error()+")")
		return store.NewUtxoCache(), true, nil
	}
	if loaded == nil {
		return store.NewUtxoCache(), false, nil
	}
	return loaded, false, nil
}

func measureStartupBodyContiguous(j *store.HeaderJournal, raw *store.RawBlockStore, net chain.Network, seed int64) int64 {
	if j == nil || raw == nil {
		return -1
	}
	return store.MeasureContiguousBodiesOnDisk(j, raw, net, seed, 16_384)
}

// shouldQuarantineFabricatedUtxoSnapshot rejects test/bogus snapshots (huge tip, empty coin map,
// almost no stored bodies). Normal UTXO-ahead replay snapshots are never quarantined.
func shouldQuarantineFabricatedUtxoSnapshot(diskTip int64, path string, checkpointCont int64, j *store.HeaderJournal, raw *store.RawBlockStore, net chain.Network) bool {
	if diskTip < 0 {
		return false
	}
	measured := measureStartupBodyContiguous(j, raw, net, checkpointCont)
	if measured >= 0 && diskTip <= measured+utxoSnapshotBodyMargin {
		return false
	}
	if j != nil && raw != nil && store.HasStoredBodyAtHeight(j, raw, diskTip, net) {
		return false
	}
	_, coinCount, err := store.ReadUtxoSnapshotTipAndCount(path)
	if err != nil {
		return false
	}
	// Real replay snapshots carry chainstate from connect; fabricated test saves do not.
	if diskTip >= 512 && coinCount >= 8 {
		return false
	}
	if diskTip > measured+utxoSnapshotBodyMargin && measured < 256 && coinCount <= 1 {
		return true
	}
	return false
}
