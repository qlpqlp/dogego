// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
)

const autoRecoverHeaderValidateWindow int64 = 2048

var autoRecoverSweepWG sync.WaitGroup

// WaitAutoRecoverSweepAsync blocks until background repair goroutines from autoRecoverSweep finish.
func WaitAutoRecoverSweepAsync() {
	autoRecoverSweepWG.Wait()
}

// autoRecoverHeaders validates recent stored headers and applies local journal
// rewinds when corruption/consensus mismatch is detected.
func autoRecoverHeaders(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx) (bool, error) {
	if j == nil || p.RelaxedPoW {
		// Reboot testnet / regtest-style chains use locally mined headers; skip full
		// mainnet-style stored validation during periodic sweeps (fixtures lack peer PoW).
		return false, nil
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return false, err
	}
	start := tip - autoRecoverHeaderValidateWindow
	if start < 0 {
		start = 0
	}
	if err := consensus.ValidateStoredHeaders(j, aux, p, start, tip, time.Now().Unix()); err != nil {
		applog.Line("headers", fmt.Sprintf("auto recovery: header validation failed in %d..%d (%v)", start, tip, err))
		rewound, rerr := runLocalHeaderJournalRecovery(j, aux, p, bs, err)
		if rewound {
			applog.Line("headers", "auto recovery: rewound header journal and will retry sync automatically")
			return true, nil
		}
		if rerr != nil && !IsHeaderRewindRetryErr(rerr) {
			return false, fmt.Errorf("auto header recovery failed: %w", rerr)
		}
		if rerr != nil {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// autoRecoverSweep performs a fast, idempotent recovery pass over headers/raw/index/filter state.
// It is safe to run on startup and periodically while the node is running.
func autoRecoverSweep(chainDir string, j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx, repairFilters func()) (bool, error) {
	var rewound bool

	if j != nil {
		if err := autoRecoverGenesisSanity(j, p); err != nil {
			applog.Line("recovery", err.Error())
		}
	}

	if chainDir != "" {
		maybeRepairHeaderAuxFile(chainDir, j)
		if n, err := store.PurgeStaleHeaderSyncTemps(chainDir); err != nil {
			applog.Line("recovery", "auto headers_sync tmp purge: "+err.Error())
		} else if n > 0 {
			applog.Line("recovery", fmt.Sprintf("auto headers_sync tmp purge: removed %d stale temp file(s)", n))
		}
		if n, err := store.PurgeStaleUtxoSnapshotTemps(chainDir); err != nil {
			applog.Line("recovery", "auto utxo.cache.tmp purge: "+err.Error())
		} else if n > 0 {
			applog.Line("recovery", fmt.Sprintf("auto utxo.cache.tmp purge: removed %d stale temp file(s)", n))
		}
	}

	rw, err := autoRecoverHeaders(j, aux, p, bs)
	if err != nil {
		applog.Line("recovery", "auto header sweep: "+err.Error())
	} else if rw {
		rewound = true
	}

	if bs != nil {
		if n, err := bs.PurgeStaleRawBlockTemps(); err != nil {
			applog.Line("recovery", "auto raw tmp purge: "+err.Error())
		} else if n > 0 {
			applog.Line("recovery", fmt.Sprintf("auto raw tmp purge: removed %d stale temp file(s)", n))
		}
		if !bs.utxoAheadOfStoredBodies() {
			if n, err := bs.PurgeInadequateRawBodies(); err != nil {
				applog.Line("recovery", "auto raw purge: "+err.Error())
			} else if n > 0 {
				applog.Line("recovery", fmt.Sprintf("auto raw purge: removed %d undersized body file(s)", n))
			}
		}
		if err := EnsureLocalGenesis(bs); err != nil {
			applog.Line("recovery", "auto local genesis: "+err.Error())
		}
		ReconcileGenesisWithContiguous(bs)
		bs.maybeClampBundledContiguousFromDisk()
		prev := bs.ContiguousRawHeight()
		refreshed := bs.RefreshContiguousTip()
		if refreshed > prev {
			applog.Line("recovery", fmt.Sprintf("auto contiguous tip %d → %d", prev, refreshed))
		}
		if chainDir != "" {
			if fixed, err := store.ReconcileRawBlockSyncCheckpoint(chainDir, bs.ContiguousRawHeight()); err != nil {
				applog.Line("recovery", "auto rawblocks_sync reconcile: "+err.Error())
			} else if fixed {
				applog.Line("recovery", "auto rawblocks_sync reconcile: clamped stale checkpoint to contiguous frontier")
			}
		}
	}

	if chainDir != "" {
		runTxIndexRepair := func() {
			maybeRepairTxIndex(chainDir, bs, txIndexRepairMinRawBlocks)
			maybeUpgradeLegacyTxIndex(chainDir, txIndexLegacyUpgradeBatch)
		}
		if bs != nil && BodiesBehindHeaders(bs) {
			autoRecoverSweepWG.Add(1)
			go func() {
				defer autoRecoverSweepWG.Done()
				runTxIndexRepair()
			}()
		} else {
			runTxIndexRepair()
		}
		if txIx, err := store.OpenTxIndex(chainDir); err == nil {
			if n, err := txIx.PurgeStaleTxIndexTemps(); err != nil {
				applog.Line("recovery", "auto tx index tmp purge: "+err.Error())
			} else if n > 0 {
				applog.Line("recovery", fmt.Sprintf("auto tx index tmp purge: removed %d stale temp file(s)", n))
			}
		}
		if filterIx, err := store.OpenBlockFilterIndex(chainDir); err == nil {
			if n, err := filterIx.PurgeStaleBlockFilterTemps(); err != nil {
				applog.Line("recovery", "auto filter tmp purge: "+err.Error())
			} else if n > 0 {
				applog.Line("recovery", fmt.Sprintf("auto filter tmp purge: removed %d stale temp file(s)", n))
			}
		}
	}
	if repairFilters != nil {
		if bs != nil && BodiesBehindHeaders(bs) {
			// Filter backfill is deferred during body IBD (catch-up worker + periodic repair).
		} else {
			repairFilters()
		}
	}

	if j != nil && bs != nil {
		if reset, err := maybeResetStuckAncientInSweep(j, aux, p, bs); reset {
			rewound = true
			if err != nil {
				applog.Line("recovery", "stuck ancient header reset: "+err.Error())
			}
		} else if err != nil {
			applog.Line("recovery", "stuck ancient header check: "+err.Error())
		}
	}

	return rewound, nil
}

func autoRecoverFilterRepairFn(j *store.HeaderJournal, chainDir string, filterIx *store.BlockFilterIndex, txIx *store.TxIndex, rbStore *store.RawBlockStore) func() {
	if j == nil || filterIx == nil || txIx == nil || rbStore == nil || chainDir == "" {
		return nil
	}
	return func() {
		maybeRepairBlockFilters(j, chainDir, txIndexRepairMinRawBlocks, func(hashLE [32]byte, blockRaw []byte) error {
			return rpc.IndexBasicBlockFilter(filterIx, hashLE, blockRaw, j, rbStore, txIx)
		})
	}
}

func autoRecoverPostRewind(bs *BlockStoreCtx, rawFill *progressiveRawState, headerCatchUpPending *bool) {
	if bs != nil {
		MaybeResetContiguousAfterHeaderRewind(bs)
	}
	if rawFill != nil && bs != nil {
		rawFill.ResetAfterChainTruncate(bs)
		rawFill.PrepareAtStartup(bs)
	}
	if headerCatchUpPending != nil {
		*headerCatchUpPending = true
	}
}

// maybeRepairHeaderAuxFile reopens headers_aux.bin so torn-tail repair runs during periodic sweeps.
func maybeRepairHeaderAuxFile(chainDir string, j *store.HeaderJournal) {
	auxPath := filepath.Join(chainDir, "headers_aux.bin")
	if _, err := os.Stat(auxPath); err != nil {
		return
	}
	hcount, err := j.Count()
	if err != nil {
		return
	}
	aux, err := store.OpenHeaderAuxJournal(auxPath, hcount)
	if err != nil {
		applog.Line("recovery", "headers_aux repair pass: "+err.Error())
		return
	}
	if err := aux.EnsureRecordCount(hcount); err != nil {
		applog.Line("recovery", "headers_aux align: "+err.Error())
	}
}

func autoRecoverGenesisSanity(j *store.HeaderJournal, p chain.Params) error {
	if j == nil {
		return nil
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		return err
	}
	h0, err := j.ReadHeaderAt(0)
	if err != nil {
		return err
	}
	if string(h0) != string(gen[:]) {
		return fmt.Errorf("auto recovery: genesis header mismatch in journal")
	}
	return nil
}
