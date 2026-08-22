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
	// earlyIBDBlockDownloadTimeout caps batch wait while fetching the first ~1000 blocks
	// (peers that only send inv/headers). After that, use Core nCalculatedDlWindow.
	earlyIBDBlockDownloadTimeout = 90 * time.Second
	// bodyIBDBlockDownloadTimeout caps one getdata while thousands of tiny bodies are in flight.
	bodyIBDBlockDownloadTimeout = 30 * time.Second
	// bodyIBDProgressDownloadTimeout extends the body cap while a lane is still delivering blocks.
	bodyIBDProgressDownloadTimeout = earlyIBDBlockDownloadTimeout
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
// Matches Core nCalculatedDlWindow (nPowTargetSpacing * (BASE + PER_PEER * other downloaders)).
// Only the first ~1000 bodies keep a shorter cap so inv-only peers rotate; after that a 60s
// cap was disconnecting peers that were still seeking ancient blocks (Core waits ~5â€“17 min).
func EffectiveBlockDownloadTimeout(bs *BlockStoreCtx, syncLanes int) time.Duration {
	others := syncLanes - 1
	if others < 0 {
		others = 0
	}
	d := BlockDownloadTimeout(others, 60)
	if bs != nil {
		cont := bs.ContiguousRawHeight()
		if cont < 1000 && d > earlyIBDBlockDownloadTimeout {
			return earlyIBDBlockDownloadTimeout
		}
		// Core nCalculatedDlWindow grows to 17â€“60 minutes with many peers. Relays that
		// disconnect leave getdata claims in that window and freeze IBD at <1 blk/min.
		if (ShouldDeferConnectForBodyDownload(bs) || ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) || (cont >= 0 && cont < 500_000)) && d > bodyIBDBlockDownloadTimeout {
			return bodyIBDBlockDownloadTimeout
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
