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
	"io"
	"net"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/p2p"
	"dogego/wire"
)

// blockAssistIdleSleep is how long the assist loop waits when the coordinator reports catch-up.
const blockAssistIdleSleep = 750 * time.Millisecond

// blockAssistDrainTimeout bounds draining unsolicited P2P messages between getdata batches.
const blockAssistDrainTimeout = 100 * time.Millisecond

// blockAssistPreFetchDrain waits for post-handshake chatter (sendcmpct, feefilter) before getdata.
const blockAssistPreFetchDrain = 350 * time.Millisecond

// blockAssistSessionIdleRotate disconnects assist peers that never deliver blocks (claimBatch spin).
const blockAssistSessionIdleRotate = 45 * time.Second

// blockAssistNoPeerLogInterval rate-limits "no peer" lines when dial candidates are exhausted.
const blockAssistNoPeerLogInterval = 30 * time.Second

// StartBlockAssist launches background goroutines that download missing raw blocks over
// dedicated outbound peers so the main session stays responsive for inv/headers/tx.
// syncWorkerCount is total parallel lanes (assist + primary); assist lane id = workerID+1.
// primaryExcl is read on each dial attempt so assist peers track primary auto-redial.
func StartBlockAssist(ctx context.Context, d net.Dialer, candidates *BlockAssistCandidates, primaryExcl *PrimaryExclude, p chain.Params, userAgent string, localServices uint64, bs *BlockStoreCtx, raw *progressiveRawState, workers int, syncWorkerCount int, scorer *BlockPeerScorer, assistReg *AssistPeerRegistry, book *AddrBook) {
	if bs == nil || raw == nil || candidates == nil || candidates.Len() == 0 {
		return
	}
	if workers < 1 {
		workers = minBlockAssistWorkers
	}
	if scorer == nil {
		scorer = NewBlockPeerScorer()
	}
	for w := 0; w < workers; w++ {
		workerID := w
		laneID := workerID + 1
		if syncWorkerCount < laneID+1 {
			syncWorkerCount = laneID + 1
		}
		go runBlockAssistWorker(ctx, d, candidates, primaryExcl, workerID, workers, p, userAgent, localServices, bs, raw, laneID, syncWorkerCount, scorer, assistReg, book)
	}
}

func runBlockAssistWorker(ctx context.Context, d net.Dialer, pool *BlockAssistCandidates, primaryExcl *PrimaryExclude, workerID, nWorkers int, p chain.Params, userAgent string, localServices uint64, bs *BlockStoreCtx, raw *progressiveRawState, laneID, syncWorkerCount int, scorer *BlockPeerScorer, assistReg *AssistPeerRegistry, book *AddrBook) {
	var lastNoPeerLog time.Time
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !raw.bodiesDownloadActive(bs) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(blockAssistIdleSleep):
			}
			continue
		}
		exclude := ""
		if primaryExcl != nil {
			exclude = primaryExcl.Addr()
		}
		candidates := scorer.CandidatesForWorker(pool.Snapshot(), exclude, workerID, nWorkers, blockFetchWantHeight(bs))
		if assistReg != nil {
			candidates = preferUnusedAssistPeers(candidates, assistReg.InUseAddrs())
		}
		if len(candidates) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		var lastErr error
		connected := false
		for _, addr := range candidates {
			RecordOutboundDialTry(book, addr)
			c, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				lastErr = err
				if p2p.ObserveDialError(addr, err) {
					applog.Line("net", "IPv6 dials disabled (network unreachable); preferring IPv4 peers")
				}
				RecordOutboundHandshakeResult(book, addr, err)
				scorer.NoteDialFailure(addr)
				continue
			}
			wrapped := net.Conn(c)
			if assistReg != nil {
				if ctr := assistReg.Register(addr, laneID); ctr != nil {
					wrapped = &countingConn{Conn: c, ctr: ctr}
				}
			}
			dv, err := Handshake(ctx, wrapped, p, userAgent, localServices)
			if err != nil {
				_ = c.Close()
				if assistReg != nil {
					assistReg.Unregister(addr)
				}
				lastErr = err
				RecordOutboundHandshakeResult(book, addr, err)
				scorer.NoteDialFailure(addr)
				continue
			}
			RecordOutboundPeerHandshake(book, scorer, addr, dv, nil)
			wantH := blockFetchWantHeight(bs)
			if wantH >= 0 && !chain.PeerLikelyHasBlock(dv.Services, dv.StartHeight, wantH) {
				_ = c.Close()
				if assistReg != nil {
					assistReg.Unregister(addr)
				}
				applog.Line("block", fmt.Sprintf("block-assist skip %s (BIP159 limited; cannot serve height %d)", addr, wantH))
				continue
			}
			applog.Line("block", fmt.Sprintf("block-assist worker %d connected to %s (lane %d/%d)", laneID-1, addr, laneID, syncWorkerCount))
			connected = true
			runBlockAssistSession(ctx, wrapped, addr, p, bs, raw, laneID, scorer, book)
			if assistReg != nil {
				assistReg.Unregister(addr)
			}
			_ = c.Close()
			break
		}
		if !connected {
			if lastErr != nil && time.Since(lastNoPeerLog) >= blockAssistNoPeerLogInterval {
				lastNoPeerLog = time.Now()
				applog.Line("block", fmt.Sprintf("block-assist worker %d: no peer: %v", laneID-1, lastErr))
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func runBlockAssistSession(ctx context.Context, conn net.Conn, addr string, p chain.Params, bs *BlockStoreCtx, raw *progressiveRawState, laneID int, scorer *BlockPeerScorer, book *AddrBook) {
	w := NewMsgWriter(conn, p.Magic)
	w.PeerAddr = addr
	var ping peerPingTracker
	sessionBlocks := 0
	sessionStart := time.Now()
	lastBodyAt := sessionStart
	var lastBodyPump time.Time
	defer func() {
		if raw != nil {
			raw.ReleaseLaneInFlight(laneID)
		}
		if sessionBlocks > 0 {
			scorer.NoteBlocksDelivered(addr, sessionBlocks)
		}
	}()
	// Drain post-handshake cmpct/feefilter before the first getdata (avoids bad magic on block read).
	drainAssistInboundFor(conn, p.Magic, w, &ping, blockAssistPreFetchDrain)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !raw.bodiesDownloadActive(bs) {
			return
		}
		if n, err := MaybePumpLaneBodyIBDDownload(ctx, w, p, bs, raw, laneID, scorer, book, &lastBodyPump); err != nil {
			if IsBenignShutdownErr(err) {
				return
			}
			if errors.Is(err, ErrBlockDownloadStall) || errors.Is(err, ErrBlockDownloadTimeout) {
				applog.Line("block", "block-assist disconnect: "+err.Error())
				return
			}
			if shouldRotatePeerForStubBlock(err) {
				penalizeStubBlockPeer(scorer, book, addr)
				applog.Line("block", fmt.Sprintf("block-assist %s sent undersized block stub - disconnecting", addr))
				return
			}
			penalizeBlockPeer(scorer, book, addr, sessionFailureHardFromFetchErr(err) || shouldRotatePeerForForwardIBDFetch(err, blockFetchWantHeight(bs)))
			if sessionFailureHardFromFetchErr(err) || shouldRotatePeerForForwardIBDFetch(err, blockFetchWantHeight(bs)) || shouldRedialPrimaryForAncientFetch(err, blockFetchWantHeight(bs)) {
				applog.Line("block", fmt.Sprintf("block-assist %s cannot serve blocks (%v) - disconnecting", addr, err))
				return
			}
			applog.Line("block", "block-assist fetch: "+err.Error())
			return
		} else if n > 0 {
			sessionBlocks += n
			lastBodyAt = time.Now()
		}
		drainAssistInboundFor(conn, p.Magic, w, &ping, blockAssistDrainTimeout)
		ping.maybePing(w)
		n, err := raw.tryFetchMissingBatches(ctx, w, p, bs, laneID, IdleFetchBatchesPerRound(bs), scorer, book)
		if err != nil {
			if IsBenignShutdownErr(err) {
				return
			}
			if errors.Is(err, ErrBlockDownloadStall) || errors.Is(err, ErrBlockDownloadTimeout) {
				applog.Line("block", "block-assist disconnect: "+err.Error())
				return
			}
			if shouldRotatePeerForStubBlock(err) {
				penalizeStubBlockPeer(scorer, book, addr)
				applog.Line("block", fmt.Sprintf("block-assist %s sent undersized block stub - disconnecting", addr))
				return
			}
			penalizeBlockPeer(scorer, book, addr, sessionFailureHardFromFetchErr(err) || shouldRotatePeerForForwardIBDFetch(err, blockFetchWantHeight(bs)))
			if shouldRedialPrimaryForAncientFetch(err, blockFetchWantHeight(bs)) || shouldRotatePeerForForwardIBDFetch(err, blockFetchWantHeight(bs)) {
				applog.Line("block", fmt.Sprintf("block-assist %s cannot serve blocks (%v) - disconnecting", addr, err))
				return
			}
			applog.Line("block", "block-assist fetch: "+err.Error())
			return
		}
		if n > 0 {
			sessionBlocks += n
			lastBodyAt = time.Now()
		}
		if n == 0 && !raw.bodiesDownloadActive(bs) {
			return
		}
		if n == 0 {
			if raw != nil && raw.laneHasActiveBatch(laneID) {
				lastBodyAt = time.Now()
				select {
				case <-ctx.Done():
					return
				case <-time.After(blockAssistIdleSleep):
				}
				continue
			}
			if time.Since(lastBodyAt) >= blockAssistSessionIdleRotate && time.Since(sessionStart) >= 10*time.Second {
				applog.Line("block", fmt.Sprintf("block-assist %s idle without blocks for %s; rotating peer", addr, blockAssistSessionIdleRotate))
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(blockAssistIdleSleep):
			}
		}
	}
}

func drainAssistInbound(conn net.Conn, magic [4]byte, w *MsgWriter, ping *peerPingTracker) {
	drainAssistInboundFor(conn, magic, w, ping, blockAssistDrainTimeout)
}

func drainAssistInboundFor(conn net.Conn, magic [4]byte, w *MsgWriter, ping *peerPingTracker, window time.Duration) {
	deadline := time.Now().Add(window)
	for {
		if time.Now().After(deadline) {
			return
		}
		remain := time.Until(deadline)
		if remain <= 0 {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(remain))
		cmd, pl, err := wire.ReadMessage(conn, magic)
		if err != nil {
			if err != io.EOF {
				return
			}
			return
		}
		switch cmd {
		case "ping":
			_ = replyPing(w, pl)
		case "pong":
			if ping != nil {
				ping.notePong(pl)
			}
		case "sendcmpct":
			_, _ = ReplySendCmpctDecline(w, pl)
		default:
			// inv, headers, tx, etc. - ignored on block-assist link
		}
	}
}

// StartBlockAssistWorkers is EnsureBlockAssistWorkers (kept for call sites).
func StartBlockAssistWorkers(ctx context.Context, d net.Dialer, candidates *BlockAssistCandidates, primaryExcl *PrimaryExclude, p chain.Params, userAgent string, localServices uint64, bs *BlockStoreCtx, raw *progressiveRawState, workers int, scorer *BlockPeerScorer, assistReg *AssistPeerRegistry, book *AddrBook) {
	EnsureBlockAssistWorkers(BlockAssistLaunchParams{
		Ctx: ctx, Dialer: d, Candidates: candidates, PrimaryExcl: primaryExcl, Params: p,
		UserAgent: userAgent, LocalServices: localServices, BlockStore: bs, Raw: raw,
		Workers: workers, Scorer: scorer, AssistReg: assistReg, AddrBook: book,
	})
}

// preferUnusedAssistPeers moves in-use assist peers to the end so parallel lanes avoid hammering one host (Core: one block sync per peer).
func preferUnusedAssistPeers(candidates []string, inUse []string) []string {
	if len(candidates) <= 1 || len(inUse) == 0 {
		return candidates
	}
	busy := make(map[string]struct{}, len(inUse))
	for _, a := range inUse {
		if a != "" {
			busy[a] = struct{}{}
		}
	}
	var fresh, used []string
	for _, a := range candidates {
		if _, ok := busy[a]; ok {
			used = append(used, a)
		} else {
			fresh = append(fresh, a)
		}
	}
	if len(fresh) == 0 {
		return candidates
	}
	return append(fresh, used...)
}

// assistPeerCandidates returns dial targets for the block-assist goroutine (DNS/fixed seeds + ranked history).
func assistPeerCandidates(ctx context.Context, p chain.Params, discovered []string, scorer *BlockPeerScorer, wantBlockHeight int64) []string {
	var base []string
	if len(discovered) > 0 {
		base = discovered
	} else {
		base = p2p.DiscoverAddresses(ctx, p, nil)
	}
	base = SpreadHostPortsByGroup16(base)
	if scorer != nil {
		return p2p.PreferIPv4First(scorer.MergeDiscoveryCandidates(base, wantBlockHeight))
	}
	return p2p.PreferIPv4First(base)
}
