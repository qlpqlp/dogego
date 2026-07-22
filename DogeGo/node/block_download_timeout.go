// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"time"
)

// Core validation.h / net_processing.cpp block download timeout (millionths of block interval).
const (
	blockDownloadTimeoutBaseMicro    = int64(5_000_000)  // ~5 min at 60s spacing
	blockDownloadTimeoutPerPeerMicro = int64(2_500_000) // +~2.5 min per other downloading peer
	blockDownloadMinMultiplierSec    = int64(10)
	// earlyIBDBlockDownloadTimeout caps batch wait while fetching ancient blocks near genesis.
	earlyIBDBlockDownloadTimeout = 90 * time.Second
	// bodyIBDPauseDownloadTimeout caps batch wait while deep forward body IBD runs (rotate slow peers faster).
	bodyIBDPauseDownloadTimeout = 60 * time.Second
	// batchBlockReadSlice caps each ReadMessage wait so ping/inv chatter cannot stall a batch until Core-scale windows.
	batchBlockReadSlice = 15 * time.Second
)

// BlockDownloadTimeout returns how long to wait on one peer for in-flight block batch progress
// before treating the session as stalled (Core nCalculatedDlWindow analogue).
func BlockDownloadTimeout(otherDownloadingPeers int, blockIntervalSec int64) time.Duration {
	if otherDownloadingPeers < 0 {
		otherDownloadingPeers = 0
	}
	if blockIntervalSec <= 0 {
		blockIntervalSec = 60
	}
	mult := blockIntervalSec
	if mult < blockDownloadMinMultiplierSec {
		mult = blockDownloadMinMultiplierSec
	}
	windowSec := mult * (blockDownloadTimeoutBaseMicro + blockDownloadTimeoutPerPeerMicro*int64(otherDownloadingPeers)) / 1_000_000
	if windowSec < 60 {
		windowSec = 60
	}
	if windowSec > 3600 {
		windowSec = 3600
	}
	return time.Duration(windowSec) * time.Second
}

// EffectiveBlockDownloadTimeout returns the in-flight batch window for progressive getdata.
// Ancient forward IBD uses a shorter cap so assist peers that only send inv/headers rotate quickly.
func EffectiveBlockDownloadTimeout(bs *BlockStoreCtx, syncLanes int) time.Duration {
	others := syncLanes - 1
	if others < 0 {
		others = 0
	}
	d := BlockDownloadTimeout(others, 60)
	if bs != nil {
		cont := bs.ContiguousRawHeight()
		if ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) && cont >= 0 && cont < 100_000 && d > bodyIBDPauseDownloadTimeout {
			return bodyIBDPauseDownloadTimeout
		}
		if cont < 1000 && d > earlyIBDBlockDownloadTimeout {
			return earlyIBDBlockDownloadTimeout
		}
	}
	return d
}

// blockBatchDeadline is the absolute wall time by which a batched getdata must finish.
func blockBatchDeadline(ctx context.Context, syncLanes int, bs *BlockStoreCtx) time.Time {
	return deadlineFromCtx(ctx, EffectiveBlockDownloadTimeout(bs, syncLanes))
}

// batchBlockReadDeadline picks a per-read deadline capped within the overall batch window.
func batchBlockReadDeadline(batchEnd time.Time) time.Time {
	slice := time.Now().Add(batchBlockReadSlice)
	if slice.After(batchEnd) {
		return batchEnd
	}
	return slice
}
