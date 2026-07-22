// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/store"
)

const (
	headerSyncBackgroundMaxWait = 5 * time.Minute
	// Core-style: don't let one background pass or one peer candidate monopolize recovery.
	headerSyncRecoveryPassTimeout      = 3 * time.Minute
	headerSyncRecoveryCandidateTimeout = 45 * time.Second
)

// HeaderSyncRecoveryEnv drives automatic header catch-up while the node stays running.
type HeaderSyncRecoveryEnv struct {
	Ctx           context.Context
	Dialer        net.Dialer
	Params        chain.Params
	SubVer        string
	LocalServices uint64
	Journal       *store.HeaderJournal
	Aux           *store.HeaderAuxJournal
	BlockStore    *BlockStoreCtx
	FeeFilters    *FeeFilterSet
	RawBackfill   int
	RawFill       *progressiveRawState
	DiscoveryFeed *PeerDiscoveryFeed
	Discovered    []string
	Scorer        *BlockPeerScorer
	AddrBook      *AddrBook
	AddedNodes    []string
	OnSuccess     func(peer headerSyncPeer)
	// RefreshDiscovery re-runs DNS/fixed-seed discovery when candidates are stale (optional).
	RefreshDiscovery func() []string
	// OnExhausted runs when the recovery loop exits without attaching a peer (optional).
	OnExhausted func(lastErr error)
}

var headerSyncBGRecoveryRunning atomic.Int32
var headerSyncBGMu sync.Mutex
var headerSyncBGCancel context.CancelFunc
var headerSyncBGRunID uint64
var headerSyncBGActiveConn struct {
	mu   sync.Mutex
	conn net.Conn
}

func setHeaderSyncBGActiveConn(c net.Conn) {
	headerSyncBGActiveConn.mu.Lock()
	headerSyncBGActiveConn.conn = c
	headerSyncBGActiveConn.mu.Unlock()
}

func clearHeaderSyncBGActiveConn() {
	headerSyncBGActiveConn.mu.Lock()
	headerSyncBGActiveConn.conn = nil
	headerSyncBGActiveConn.mu.Unlock()
}

// StartHeaderSyncBackgroundRecovery retries header sync with local journal rewinds until success or ctx cancel.
func StartHeaderSyncBackgroundRecovery(env HeaderSyncRecoveryEnv) {
	StartHeaderSyncBackgroundRecoveryOnce(env)
}

// StartHeaderSyncBackgroundRecoveryOnce starts at most one background recovery goroutine.
func StartHeaderSyncBackgroundRecoveryOnce(env HeaderSyncRecoveryEnv) bool {
	if !headerSyncBGRecoveryRunning.CompareAndSwap(0, 1) {
		return false
	}
	runCtx, cancel := context.WithCancel(env.Ctx)
	headerSyncBGMu.Lock()
	headerSyncBGRunID++
	runID := headerSyncBGRunID
	headerSyncBGCancel = cancel
	headerSyncBGMu.Unlock()
	go func() {
		var lastExit error
		defer func() {
			headerSyncBGMu.Lock()
			if headerSyncBGRunID == runID {
				headerSyncBGCancel = nil
			}
			headerSyncBGMu.Unlock()
			headerSyncBGRecoveryRunning.Store(0)
			if env.OnExhausted != nil {
				env.OnExhausted(lastExit)
			}
		}()
		env.Ctx = runCtx
		lastExit = runHeaderSyncBackgroundRecovery(env)
	}()
	return true
}

// ForceRestartHeaderSyncBackgroundRecovery cancels the currently running background recovery pass.
// Caller should schedule StartHeaderSyncBackgroundRecoveryOnce shortly after this returns true.
func ForceRestartHeaderSyncBackgroundRecovery() bool {
	headerSyncBGMu.Lock()
	cancel := headerSyncBGCancel
	headerSyncBGMu.Unlock()
	if cancel == nil || headerSyncBGRecoveryRunning.Load() == 0 {
		return false
	}
	cancel()
	headerSyncBGActiveConn.mu.Lock()
	if c := headerSyncBGActiveConn.conn; c != nil {
		_ = c.Close()
	}
	headerSyncBGActiveConn.mu.Unlock()
	return true
}

func runHeaderSyncBackgroundRecovery(env HeaderSyncRecoveryEnv) error {
	wait := headerSyncBackgroundInitial
	pass := 0
	var lastSyncErr error
	for {
		select {
		case <-env.Ctx.Done():
			return env.Ctx.Err()
		case <-time.After(wait):
		}
		pass++
		NoteHeaderRecoveryPass(pass, "waiting for peers or retry interval")
		ok, peer, lastErr := tryHeaderSyncRecoveryPass(env, pass, lastSyncErr)
		if lastErr != nil {
			lastSyncErr = lastErr
			if shouldAutoRecoverHeaderSync(lastErr) {
				noteHeaderSyncFailure(lastErr)
			}
		}
		if ok && peer != nil && env.OnSuccess != nil {
			env.OnSuccess(*peer)
			return nil
		}
		if lastErr != nil && !shouldAutoRecoverHeaderSync(lastErr) {
			applog.Line("headers", fmt.Sprintf("background header recovery stopped (non-recoverable): %v", lastErr))
			return lastErr
		}
		if wait < headerSyncBackgroundMaxWait {
			wait += 15 * time.Second
			if wait > headerSyncBackgroundMaxWait {
				wait = headerSyncBackgroundMaxWait
			}
		}
	}
}

func tryHeaderSyncRecoveryPass(env HeaderSyncRecoveryEnv, pass int, lastSyncErr error) (bool, *headerSyncPeer, error) {
	if env.Journal == nil {
		return false, nil, nil
	}
	if env.BlockStore != nil && ShouldPauseHeaderCatchUpForBodyIBD(env.BlockStore, 0) {
		return false, nil, nil
	}
	passCtx, cancelPass := context.WithTimeout(env.Ctx, headerSyncRecoveryPassTimeout)
	defer cancelPass()
	ok, recErr := runLocalHeaderJournalRecovery(env.Journal, env.Aux, env.Params, env.BlockStore, lastSyncErr)
	if recErr != nil {
		return false, nil, recErr
	}
	if ok && env.BlockStore != nil {
		MaybeResetContiguousAfterHeaderRewind(env.BlockStore)
	}
	if ok {
		applog.Line("headers", fmt.Sprintf("background header recovery %d: pruned stale headers/blocks - probing peers", pass))
	}
	discovered := env.Discovered
	if env.RefreshDiscovery != nil && recoveryShouldRefreshDiscovery(pass, -1) {
		if fresh := env.RefreshDiscovery(); len(fresh) > 0 {
			discovered = fresh
			env.Discovered = fresh
		}
	}
	peers := HeaderSyncProbeCandidates(DiscoverySnapshot(env.DiscoveryFeed, discovered), env.Scorer, env.AddedNodes)
	if len(peers) == 0 && env.RefreshDiscovery != nil {
		if fresh := env.RefreshDiscovery(); len(fresh) > 0 {
			discovered = fresh
			env.Discovered = fresh
			peers = HeaderSyncProbeCandidates(DiscoverySnapshot(env.DiscoveryFeed, discovered), env.Scorer, env.AddedNodes)
		}
	}
	if len(peers) == 0 {
		return false, nil, fmt.Errorf("no peer candidates for header recovery")
	}
	probed, probeErr := probeHeaderSyncPeers(passCtx, env.Dialer, peers, env.Params, env.SubVer, env.LocalServices, headerSyncPeerProbeMax, env.Scorer, env.AddrBook)
	if probeErr != nil {
		return false, nil, probeErr
	}
	if len(probed) == 0 {
		return false, nil, fmt.Errorf("no peers handshook for header recovery")
	}
	defer func() {
		for _, p := range probed {
			closeHeaderSyncPeer(p)
		}
	}()
	var lastErr error
	for i, peer := range probed {
		applog.Line("headers", fmt.Sprintf("background header sync with %s (candidate %d/%d)", peer.addr, i+1, len(probed)))
		NoteHeaderRecoveryPass(pass, fmt.Sprintf("downloading from %s", peer.addr))
		startTip, _ := env.Journal.TipHeight()
		candidateCtx, cancelCandidate := context.WithTimeout(passCtx, headerSyncRecoveryCandidateTimeout)
		if peer.conn != nil {
			setHeaderSyncBGActiveConn(peer.conn)
		}
		// Headers-only on the recovery link: block bodies use block-assist / primary getdata.
		// Interleaving progressive getdata here can block for minutes on the same TCP session.
		err := DownloadHeaders(candidateCtx, peer.mw, env.Params, env.Journal, env.Aux, env.FeeFilters, env.BlockStore, env.RawBackfill, env.RawFill, peer.startHeight(), env.DiscoveryFeed, true, env.Scorer, env.AddrBook)
		clearHeaderSyncBGActiveConn()
		cancelCandidate()
		if errors.Is(err, context.DeadlineExceeded) {
			endTip, _ := env.Journal.TipHeight()
			if endTip <= startTip {
				err = fmt.Errorf("timeout waiting for headers: background candidate %s made no header progress in %s", peer.addr, headerSyncRecoveryCandidateTimeout)
			}
		}
		if err == nil {
			win := peer
			for j, other := range probed {
				if j != i {
					closeHeaderSyncPeer(other)
				}
			}
			// winner keeps connection open - do not close in defer
			probed[i] = headerSyncPeer{}
			return true, &win, nil
		}
		lastErr = err
		if shouldTryNextHeaderSyncPeer(err) {
			noteHeaderSyncPeerFailure(env.Scorer, env.AddrBook, peer.addr, err)
			if IsHeaderRewindRetryErr(err) && env.BlockStore != nil {
				MaybeResetContiguousAfterHeaderRewind(env.BlockStore)
			}
			applog.Line("headers", fmt.Sprintf("background: peer %s (%v); trying next candidate", peer.addr, err))
			continue
		}
		return false, nil, err
	}
	if errors.Is(passCtx.Err(), context.DeadlineExceeded) {
		return false, nil, fmt.Errorf("timeout waiting for headers: background recovery pass exceeded %s", headerSyncRecoveryPassTimeout)
	}
	return false, nil, lastErr
}

func recoveryShouldRefreshDiscovery(pass int, candidateCount int) bool {
	if candidateCount == 0 {
		return true
	}
	return pass == 1 || pass%4 == 0
}
