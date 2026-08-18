// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"net"
	"sync"

	"dogego/applog"
	"dogego/chain"
)

// BlockAssistLaunchParams holds inputs for starting parallel block download workers (one launch per process).
type BlockAssistLaunchParams struct {
	Ctx           context.Context
	Dialer        net.Dialer
	Candidates    *BlockAssistCandidates
	PrimaryExcl   *PrimaryExclude
	Params        chain.Params
	UserAgent     string
	LocalServices uint64
	BlockStore    *BlockStoreCtx
	Raw           *progressiveRawState
	Workers       int
	Scorer        *BlockPeerScorer
	AssistReg     *AssistPeerRegistry
	AddrBook      *AddrBook
}

var (
	blockAssistLaunchMu sync.Mutex
	blockAssistActive   bool
)

// BlockAssistWorkersActive reports whether block-assist goroutines were started.
func BlockAssistWorkersActive() bool {
	blockAssistLaunchMu.Lock()
	defer blockAssistLaunchMu.Unlock()
	return blockAssistActive
}

// ResetBlockAssistLaunchForTests clears the launch latch (tests only).
func ResetBlockAssistLaunchForTests() {
	resetBlockAssistLaunch()
}

func resetBlockAssistLaunch() {
	blockAssistLaunchMu.Lock()
	defer blockAssistLaunchMu.Unlock()
	blockAssistActive = false
}

// EnsureBlockAssistWorkers sets sync lane count and starts assist goroutines when peers exist.
// Safe to call after IBD stall peer refresh when the initial pool was empty (second call starts workers).
func EnsureBlockAssistWorkers(p BlockAssistLaunchParams) {
	if p.BlockStore == nil || p.Raw == nil || p.BlockStore.Raw == nil {
		return
	}
	if p.Scorer == nil {
		p.Scorer = NewBlockPeerScorer()
	}
	if p.Candidates == nil {
		p.Candidates = NewBlockAssistCandidates(assistPeerCandidates(p.Ctx, p.Params, nil, p.Scorer, blockFetchWantHeight(p.BlockStore)), p.Scorer)
	} else if p.Candidates.Len() == 0 {
		p.Candidates.Refresh(assistPeerCandidates(p.Ctx, p.Params, nil, p.Scorer, blockFetchWantHeight(p.BlockStore)), nil, p.Scorer)
	}
	workers := p.Workers
	if workers < 1 {
		workers = minBlockAssistWorkers
	}
	syncLanes := workers + 1
	p.Raw.SetSyncParallelism(syncLanes)
	if p.Candidates.Len() == 0 {
		return
	}
	if BlockDownloadDeferredForHeaders(p.BlockStore) {
		applog.Line("block", "block-assist deferred until headers.bin reaches a plausible mainnet tip (ancient partial chain was reset at startup)")
		return
	}
	blockAssistLaunchMu.Lock()
	defer blockAssistLaunchMu.Unlock()
	if blockAssistActive {
		return
	}
	blockAssistActive = true
	applog.Line("block", fmt.Sprintf("starting %d block-assist worker(s), %d parallel sync lane(s) (%d ranked peers)", workers, syncLanes, p.Candidates.Len()))
	StartBlockAssist(p.Ctx, p.Dialer, p.Candidates, p.PrimaryExcl, p.Params, p.UserAgent, p.LocalServices, p.BlockStore, p.Raw, workers, syncLanes, p.Scorer, p.AssistReg, p.AddrBook)
}
