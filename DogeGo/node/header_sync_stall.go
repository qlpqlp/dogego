// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/store"
)

const (
	headerSyncStallEarlyIBD     = 25 * time.Second
	headerSyncStallDefault      = 90 * time.Second
	headerAdvanceWatchInterval  = 40 * time.Second
	headerTopUpDuringIBD        = 45 * time.Second
	headerSyncBackgroundInitial = 5 * time.Second
)

// Core net_processing.h HEADERS_DOWNLOAD_TIMEOUT_* (microseconds).
const (
	headersDownloadTimeoutBaseUs      = 15 * 60 * 1_000_000
	headersDownloadTimeoutPerHeaderUs = 1000
)

// headersSyncTimeoutFromCore matches Core's initial-headers-sync stall window when the
// header tip block time is more than 24h behind adjusted network time.
func headersSyncTimeoutFromCore(headerTipBlockTime, nowUnix, blockIntervalSec int64) time.Duration {
	if headerTipBlockTime <= 0 || nowUnix <= 0 {
		return headerSyncStallDefault
	}
	if nowUnix-headerTipBlockTime < 24*60*60 {
		return headerSyncStallDefault
	}
	if blockIntervalSec <= 0 {
		blockIntervalSec = 60
	}
	age := nowUnix - headerTipBlockTime
	us := headersDownloadTimeoutBaseUs + headersDownloadTimeoutPerHeaderUs*age/blockIntervalSec
	d := time.Duration(us) * time.Microsecond
	if d < headerSyncStallDefault {
		return headerSyncStallDefault
	}
	if d > 2*time.Hour {
		return 2 * time.Hour
	}
	return d
}

func headerTipBlockTimeUnix(j *store.HeaderJournal, tip int64) int64 {
	if j == nil || tip < 0 {
		return 0
	}
	h80, err := j.ReadHeaderAt(tip)
	if err != nil || len(h80) < 72 {
		return 0
	}
	return int64(binary.LittleEndian.Uint32(h80[68:72]))
}

// headerSyncStallLimit is how long we wait on one TCP link without a headers message before rotating peers.
func headerSyncStallLimit(localTip int64, peerStart int32, bodiesBehind bool, headerTipTime, nowUnix int64) time.Duration {
	if localTip < 1_000_000 && peerStart > 0 && int64(peerStart) > localTip+headerCatchUpPeerLead {
		return headerSyncStallEarlyIBD
	}
	if bodiesBehind && peerStart > 0 && int64(peerStart) > localTip+headerCatchUpPeerLead {
		return headerSyncStallEarlyIBD
	}
	// Early IBD: header block times are ancient by definition; Core's 24h+ formula would allow
	// multi-hour stalls on a link that should rotate in under a minute (background recovery uses 45s).
	if localTip < 1_000_000 {
		return headerSyncStallDefault
	}
	if headerTipTime > 0 && nowUnix > 0 {
		if core := headersSyncTimeoutFromCore(headerTipTime, nowUnix, 60); core > headerSyncStallDefault {
			return core
		}
	}
	return headerSyncStallDefault
}

func isHeaderSyncYieldForBackground(err error) bool {
	return err != nil && strings.Contains(err.Error(), "background catch-up required")
}

// shouldContinueHeaderCatchUpDuringIBD reports whether the header tip is still far behind the network.
func shouldContinueHeaderCatchUpDuringIBD(j *store.HeaderJournal, peerStart int32) bool {
	if j == nil {
		return false
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 1 {
		return false
	}
	if peerStart <= 0 {
		// Unknown network tip (no peer yet): do not assume headers lag. Body IBD and initial
		// header sync on connect handle progress; latching catch-up here starves block download.
		return false
	}
	return int64(peerStart) > tip+headerCatchUpPeerLead
}

// StartHeaderAdvanceWatchdog kicks background header sync when the journal tip stops advancing during IBD.
func StartHeaderAdvanceWatchdog(ctx context.Context, net chain.Network, j *store.HeaderJournal, peerStartHeight func() int32, shouldCatchUp func(*store.HeaderJournal, int32) bool, kick func()) {
	if j == nil || kick == nil {
		return
	}
	if shouldCatchUp == nil {
		shouldCatchUp = func(j *store.HeaderJournal, peerH int32) bool {
			return shouldContinueHeaderCatchUpDuringIBD(j, peerH)
		}
	}
	go func() {
		ticker := time.NewTicker(headerAdvanceWatchInterval)
		defer ticker.Stop()
		var lastTip int64 = -1
		var lastAdvance time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				j.ReconcileCountCacheFromDisk()
				tip, err := j.TipHeight()
				if err != nil {
					continue
				}
				peerH := int32(0)
				if peerStartHeight != nil {
					peerH = peerStartHeight()
				}
				if !shouldCatchUp(j, peerH) {
					lastTip = tip
					lastAdvance = time.Now()
					continue
				}
				if tip != lastTip {
					lastTip = tip
					lastAdvance = time.Now()
					continue
				}
				if lastAdvance.IsZero() {
					lastAdvance = time.Now()
					continue
				}
				if time.Since(lastAdvance) < headerAdvanceWatchInterval {
					continue
				}
				if ShouldDeferBackgroundHeaderSync() {
					lastAdvance = time.Now()
					continue
				}
				if logNow, _ := RecordWatchdogHeaderStall(tip, peerH); logNow {
					applog.Line("headers", fmt.Sprintf("header advance watchdog: tip stuck at %d (peer height %d) - starting background header sync", tip, peerH))
					maybeNotePostAuxEraHeaderStall(net, tip)
				}
				kick()
				lastAdvance = time.Now()
			}
		}
	}()
}
