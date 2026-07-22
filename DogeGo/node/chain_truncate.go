// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"path/filepath"
	"time"

	"dogego/applog"
	"dogego/store"
)

// TruncateChainToHeight removes headers above keepThrough and prunes raw blocks, tx index, and UTXO state.
func TruncateChainToHeight(j *store.HeaderJournal, aux *store.HeaderAuxJournal, bs *BlockStoreCtx, keepThrough int64) error {
	return withHeaderChainWriteErr(func() error {
		return truncateChainToHeightLocked(j, aux, bs, keepThrough)
	})
}

func truncateChainToHeightLocked(j *store.HeaderJournal, aux *store.HeaderAuxJournal, bs *BlockStoreCtx, keepThrough int64) error {
	if j == nil {
		return fmt.Errorf("truncate: nil journal")
	}
	j.ReconcileCountCacheFromDisk()
	tipH, err := j.TipHeight()
	if err != nil {
		return err
	}
	if keepThrough >= tipH {
		return nil
	}
	var raw *store.RawBlockStore
	var txIx *store.TxIndex
	if bs != nil {
		raw = bs.Raw
		txIx = bs.TxIndex
		if bs.OnChainTruncating != nil {
			bs.OnChainTruncating(keepThrough)
		}
	}
	contiguous := int64(-1)
	if bs != nil {
		contiguous = bs.ContiguousRawHeight()
	}
	if shouldPurgeBodiesOnHeaderRewind(tipH, keepThrough, contiguous) && raw != nil {
		if n, err := store.PruneChainDataAboveHeight(j, raw, txIx, 0); err != nil {
			return fmt.Errorf("truncate purge bodies: %w", err)
		} else if n > 0 {
			applog.Line("headers", fmt.Sprintf("truncate: dropped %d raw block(s) above genesis (header rewind with corrupt/partial bodies)", n))
			bs.ResetContiguousTip()
		}
	}
	if n, err := store.PruneChainDataAboveHeight(j, raw, txIx, keepThrough); err != nil {
		return fmt.Errorf("truncate prune: %w", err)
	} else if n > 0 {
		applog.Line("headers", fmt.Sprintf("truncate: pruned %d raw block(s) above height %d", n, keepThrough))
		if bs != nil {
			bs.ResetContiguousTip()
		}
	}
	if bs != nil && bs.Journal != nil {
		chainDir := filepath.Dir(bs.Journal.Path())
		probe := keepThrough + 1
		if probe < 0 {
			probe = 0
		}
		if err := store.SaveRawBlockSyncCheckpoint(chainDir, store.RawBlockSyncCheckpoint{NextProbeHeight: probe}); err != nil {
			applog.Line("block", "truncate checkpoint: "+err.Error())
		}
	}
	if err := j.TruncateToHeight(keepThrough); err != nil {
		return err
	}
	if bs != nil && bs.ChainWork != nil {
		bs.ChainWork.Invalidate()
		bs.ChainWork.Warm(j)
	}
	if aux != nil {
		want := keepThrough + 1
		if err := aux.EnsureRecordCount(want); err != nil {
			return fmt.Errorf("truncate aux: %w", err)
		}
	}
	if bs != nil && bs.Utxo != nil && raw != nil && bs.TxIndex != nil {
		cont := bs.ContiguousRawHeight()
		if cont >= 0 && cont < keepThrough {
			applog.Line("utxo", fmt.Sprintf("truncate: defer UTXO rebuild through %d (contiguous bodies through %d)", keepThrough, cont))
		} else if err := bs.RebuildUtxoThrough(keepThrough); err != nil {
			applog.Line("utxo", "truncate rebuild: "+err.Error())
		} else if bs.Journal != nil {
			chainDir := filepath.Dir(bs.Journal.Path())
			if err := bs.Utxo.SaveSnapshot(store.UtxoSnapshotPath(chainDir)); err != nil {
				applog.Line("utxo", "truncate snapshot save: "+err.Error())
			}
		}
	}
	if bs != nil {
		if n, err := bs.PurgeInadequateRawBodies(); err != nil {
			applog.Line("block", "truncate purge stubs: "+err.Error())
		} else if n > 0 {
			applog.Line("block", fmt.Sprintf("truncate: removed %d undersized raw block stub(s)", n))
		}
		if bs.Raw != nil {
			bs.Raw.ReconcileCountCacheFromDisk()
		}
		if bs.OnChainTruncated != nil {
			keep := keepThrough
			fn := bs.OnChainTruncated
			// Run after headerChainWriteMu is released (ApplyHeadersMessage holds it during bad nBits rewind).
			time.AfterFunc(0, func() { fn(keep) })
		}
	}
	return nil
}
