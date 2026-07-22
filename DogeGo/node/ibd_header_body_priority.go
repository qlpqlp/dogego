// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strconv"
	"sync/atomic"
	"time"

	"dogego/applog"
	"dogego/store"
)

// reconcileHeaderCatchUpPending clears header catch-up when deep body IBD should own the pipeline.
// Core continues block download while headers are already far ahead of stored bodies; DogeGo must not
// keep headerCatchUpPending latched from startup (peerStart=0 reads as "headers behind network").
func reconcileHeaderCatchUpPending(bs *BlockStoreCtx, pending *atomic.Bool, raw *progressiveRawState) {
	if bs == nil || pending == nil {
		return
	}
	if !ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		return
	}
	if pending.Load() {
		cont := bs.ContiguousRawHeight()
		tip, _ := bs.Journal.TipHeight()
		applog.Line("headers", "body IBD owns sync pipeline (headers through "+strconv.FormatInt(tip, 10)+
			", bodies through "+strconv.FormatInt(cont, 10)+") - deferring header catch-up")
		pending.Store(false)
		if raw != nil {
			raw.ensureBodyDownloadArmed(bs)
		}
	}
}

// headerCatchUpResumeCooldown limits re-arming header sync when body pause was never latched (restart with partial headers).
const headerCatchUpResumeCooldown = 10 * time.Minute

// MaybeResumeHeaderCatchUpAfterBodyIBD re-arms header sync when forward body IBD no longer pauses
// getheaders but local headers still trail the network (e.g. 534k partial sync → mainnet tip).
func MaybeResumeHeaderCatchUpAfterBodyIBD(
	j *store.HeaderJournal,
	bs *BlockStoreCtx,
	peerStart int32,
	wasBodyIBDPaused *bool,
	pending *atomic.Bool,
	lastKick *time.Time,
	kick func(force bool) bool,
) bool {
	if j == nil || bs == nil || wasBodyIBDPaused == nil || pending == nil || kick == nil {
		return false
	}
	paused := ShouldPauseHeaderCatchUpForBodyIBD(bs, peerStart)
	wasPaused := *wasBodyIBDPaused
	*wasBodyIBDPaused = paused
	if paused {
		return false
	}
	if !shouldContinueHeaderCatchUpDuringIBD(j, peerStart) {
		return false
	}
	if pending.Load() {
		return false
	}
	transition := wasPaused && !paused
	if !transition {
		if lastKick != nil && !lastKick.IsZero() && time.Since(*lastKick) < headerCatchUpResumeCooldown {
			return false
		}
	}
	tip, _ := j.TipHeight()
	pending.Store(true)
	ok := kick(false)
	if ok {
		if lastKick != nil {
			*lastKick = time.Now()
		}
		if transition {
			applog.Line("headers", "body IBD pause lifted at header "+strconv.FormatInt(tip, 10)+
				" - resuming header catch-up toward network height "+strconv.FormatInt(int64(peerStart), 10))
		} else {
			applog.Line("headers", "local header tip "+strconv.FormatInt(tip, 10)+
				" behind network "+strconv.FormatInt(int64(peerStart), 10)+" - arming header catch-up")
		}
	}
	return ok
}
