// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"strings"

	"dogego/applog"
	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

// JournalFailsKnownCheckpoint returns true when headers.bin disagrees with a Core checkpoint at or below tip.
func JournalFailsKnownCheckpoint(j *store.HeaderJournal, net chain.Network) (bool, string) {
	if j == nil || net != chain.MainnetDogecoin {
		return false, ""
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return false, ""
	}
	var failAt int64 = -1
	var failMsg string
	for _, cp := range chain.HeaderCheckpointsFor(net) {
		if cp.Height > tip {
			continue
		}
		h80, err := j.ReadHeaderAt(cp.Height)
		if err != nil {
			return false, ""
		}
		want := strings.ToLower(strings.TrimPrefix(cp.HashHex, "0x"))
		got := strings.ToLower(pow.BlockHashHex(h80))
		if got != want {
			failAt = cp.Height
			failMsg = fmt.Sprintf(
				"headers.bin checkpoint mismatch at height %d (got %s… want %s…) - wrong chain or corrupt journal",
				cp.Height, got[:12], want[:12],
			)
		}
	}
	if failAt < 0 {
		return false, ""
	}
	return true, failMsg
}

// MaybeResetCriticallyStaleHeadersAtStartup truncates when a known checkpoint hash does not match the journal.
func MaybeResetCriticallyStaleHeadersAtStartup(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx) (bool, error) {
	bad, reason := JournalFailsKnownCheckpoint(j, p.Net)
	if !bad {
		return false, nil
	}
	tip, _ := j.TipHeight()
	applog.Line("headers", "CRITICAL: "+reason)
	if err := TruncateChainToHeight(j, aux, bs, 0); err != nil {
		return false, err
	}
	if bs != nil && bs.ChainWork != nil {
		bs.ChainWork.Invalidate()
		bs.ChainWork.Warm(j)
	}
	newTip, _ := j.TipHeight()
	applog.Line("headers", fmt.Sprintf("header journal reset to height %d (was %d); header sync will refetch from peers", newTip, tip))
	setHeaderSyncRecoveryHint("Checkpoint mismatch in headers.bin was cleared - syncing headers from peers.")
	return true, nil
}

// BlockDownloadDeferredForHeaders is false; block download proceeds while headers catch up (Core-style parallel IBD).
func BlockDownloadDeferredForHeaders(bs *BlockStoreCtx) bool {
	return false
}
