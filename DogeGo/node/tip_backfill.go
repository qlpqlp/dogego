// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dogego/applog"
	"dogego/chain"
)

const tipBackfillRetryInterval = 3 * time.Minute

// tipBackfillCoordinator runs startup tip-window getdata once forward IBD is close enough (Core-style).
type tipBackfillCoordinator struct {
	mu         sync.Mutex
	maxHeights int
	deferred   bool
	done       bool
	lastTry    time.Time
}

func newTipBackfillCoordinator(maxHeights int, deferred bool) *tipBackfillCoordinator {
	return &tipBackfillCoordinator{
		maxHeights: maxHeights,
		deferred:   deferred,
		done:       !deferred || maxHeights <= 0,
	}
}

func (c *tipBackfillCoordinator) noteStartupRan() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.deferred {
		c.done = true
	}
}

// maybeRunDeferred fetches the tip-aligned raw block window when contiguous bodies are within tipBackfillDeferGap of the header tip.
func (c *tipBackfillCoordinator) maybeRunDeferred(ctx context.Context, mw *MsgWriter, p chain.Params, bs *BlockStoreCtx) {
	if c == nil || mw == nil || bs == nil || bs.Journal == nil {
		return
	}
	c.mu.Lock()
	if c.done || !c.deferred || c.maxHeights <= 0 {
		c.mu.Unlock()
		return
	}
	if !c.lastTry.IsZero() && time.Since(c.lastTry) < tipBackfillRetryInterval {
		c.mu.Unlock()
		return
	}
	c.lastTry = time.Now()
	maxH := c.maxHeights
	c.mu.Unlock()

	tip, err := bs.Journal.TipHeight()
	if err != nil || tip < 1 {
		return
	}
	cont := bs.ContiguousRawHeight()
	if ShouldDeferTipBackfill(tip, cont) {
		return
	}

	applog.Line("block", fmt.Sprintf("deferred tip backfill: contiguous raw through %d, fetching tip window (max %d heights)", cont, maxH))
	SyncRecentRawBlocks(ctx, mw, p, bs, maxH)

	c.mu.Lock()
	c.done = true
	c.mu.Unlock()

	if err := bs.SyncUtxoCache(); err != nil {
		applog.Line("utxo", "after deferred tip backfill: "+err.Error())
	}
}
