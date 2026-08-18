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
	"os"
	"strings"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/clock"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// rawBlockTipFetchPace is a short sleep between sequential getdata block fetches so a single
// outbound peer is less likely to drop the connection (large mainnet blocks + bursty traffic).
const rawBlockTipFetchPace = 15 * time.Millisecond

// DownloadHeaders performs headers-first sync on one P2P link. When headersOnly is true (parallel
// assist peers), block getdata interleave on that link is disabled so assist links stay header-only.
// If feeSink is non-nil, inbound feefilter messages during sync update it (BIP133-style rate in koinu per kB).
// If rbStore is non-nil and tipBackfill > 0, we optionally fetch the genesis raw block during long
// header sync (once). Tip-window backfill is deferred to after header sync (see node.Run) so we do
// not interleave thousands of getdata round-trips on the same TCP stream while the peer is still
// sending 2k-header batches - that pattern reliably triggers EOF / connection aborts.
func DownloadHeaders(ctx context.Context, w *MsgWriter, p chain.Params, j *store.HeaderJournal, aux *store.HeaderAuxJournal, feeSink *FeeFilterSet, bs *BlockStoreCtx, tipBackfill int, rawSync *progressiveRawState, peerStartHeight int32, feed *PeerDiscoveryFeed, headersOnly bool, headerScorer *BlockPeerScorer, book *AddrBook) error {
	conn := w.Conn()
	var zero [32]byte
	var didGenesisInterleave bool
	if j != nil {
		j.ReconcileCountCacheFromDisk()
	}
	localTip, _ := j.TipHeight()
	bodiesBehind := !headersOnly && BodiesBehindHeaders(bs)
	if rawSync != nil && bs != nil && !headersOnly {
		rawSync.PrepareAtStartup(bs)
	}
	if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, peerStartHeight) {
		cont := bs.ContiguousRawHeight()
		applog.Line("headers", fmt.Sprintf("forward block IBD owns pipeline (header tip %d, bodies through %d) - deferring getheaders", localTip, cont))
		return nil
	}
	if ShouldDeferHeaderSyncWhileBodiesLag(localTip, peerStartHeight, bodiesBehind) {
		cont := int64(-1)
		if bs != nil {
			cont = bs.ContiguousRawHeight()
		}
		applog.Line("headers", fmt.Sprintf("prioritizing block download (header tip %d, peer height %d, contiguous bodies through %d); deferring getheaders this session", localTip, peerStartHeight, cont))
		return nil
	}
	maxRounds := headersSyncMaxRounds(localTip, peerStartHeight, bodiesBehind)
	if localTip > 0 && bodiesBehind {
		applog.Line("block", fmt.Sprintf("headers at height %d with block bodies behind - block-assist + forward getdata run in parallel with header catch-up (max header rounds %d)", localTip, maxRounds))
		if feed != nil {
			if err := w.Write("getaddr", nil); err != nil {
				applog.Line("net", "getaddr during header sync: "+err.Error())
			}
		}
	}
	tryInterleaveGenesisOnly := func() {
		if bs == nil || bs.Raw == nil || tipBackfill <= 0 || didGenesisInterleave || !NeedsGenesisBlock(bs) {
			if bs != nil && !NeedsGenesisBlock(bs) {
				didGenesisInterleave = true
			}
			return
		}
		applog.Line("block", "interleaved: fetching genesis raw block during header sync")
		if err := SyncGenesisRawBlock(ctx, w, p, bs); err != nil {
			applog.Line("block", "interleaved genesis: "+err.Error())
		}
		didGenesisInterleave = true
	}
	// Count inv noise between headers batches (logged once when the next headers message arrives).
	var invSinceHeaders int
	var invEntriesSinceHeaders int
	var lastHeadersReceived time.Time
	headerTipTime := int64(0)
	if localTip >= 0 && j != nil {
		headerTipTime = headerTipBlockTimeUnix(j, localTip)
	}
	nowUnix := time.Now().Unix()
	if bs != nil {
		nowUnix = bs.NetworkTimeUnix()
	}
	stallLimit := headerSyncStallLimit(localTip, peerStartHeight, bodiesBehind, headerTipTime, nowUnix)
outer:
	for round := 0; round < maxRounds; round++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		loc, err := j.BuildBlockLocator(101)
		if err != nil {
			return err
		}
		payload, err := wire.EncodeGetHeaders(p.ProtocolVersion, loc, zero)
		if err != nil {
			return err
		}
		if err := w.Write("getheaders", payload); err != nil {
			return err
		}
		tipH, _ := j.TipHeight()
		applog.Line("headers", fmt.Sprintf("sent getheaders (sync round %d, locator %d hashes, local tip height %d)", round, len(loc), tipH))
		for inner := 0; inner < 100000; inner++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if !lastHeadersReceived.IsZero() {
				tipNow, _ := j.TipHeight()
				if since := time.Since(lastHeadersReceived); since > stallLimit {
					return fmt.Errorf("header sync stall: no headers for %s at tip %d (peer height %d)",
						since.Round(time.Second), tipNow, peerStartHeight)
				}
			}
			readWait := 45 * time.Second
			if !headersOnly && bodiesBehind && rawSync != nil && bs != nil {
				readWait = 5 * time.Second
			}
			_ = conn.SetReadDeadline(deadlineFromCtx(ctx, readWait))
			cmd, pl, err := wire.ReadMessage(conn, p.Magic)
			if err != nil {
				if IsBenignShutdownErr(err) {
					return err
				}
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					if !headersOnly && bodiesBehind && rawSync != nil && bs != nil {
						total, ferr := rawSync.tryFetchMissingBatches(ctx, w, p, bs, 0, 3, headerScorer, book)
						if ferr != nil && !IsBenignShutdownErr(ferr) {
							applog.Line("block", "header-sync interleaved fetch: "+ferr.Error())
						}
						if total > 0 {
							applog.Line("block", fmt.Sprintf("header-sync interleaved: %d block(s) on primary while waiting for headers", total))
						}
						continue
					}
					return fmt.Errorf("timeout waiting for headers")
				}
				return err
			}
			switch cmd {
			case "ping":
				if err := w.Write("pong", pl); err != nil {
					return err
				}
				continue
			case "headers":
				nowUnix := clock.UnixNow()
				if bs != nil {
					nowUnix = bs.NetworkTimeUnix()
				}
				count, partial, err := ApplyHeadersMessage(j, aux, p, pl, nowUnix, bs)
				if err != nil {
					if IsHeaderRewindRetryErr(err) {
						applog.Line("headers", err.Error())
						if bs != nil {
							MaybeResetContiguousAfterHeaderRewind(bs)
						}
						lastHeadersReceived = time.Time{}
						continue outer
					}
					return err
				}
				if count == 0 {
					invSinceHeaders = 0
					invEntriesSinceHeaders = 0
					tipNow, _ := j.TipHeight()
					// Core: empty headers means peer has no more past our locator - only valid when peer is near our tip.
					if peerStartHeight > 0 && int64(peerStartHeight) > tipNow+headerCatchUpPeerLead {
						return fmt.Errorf("peer returned empty headers at tip %d (peer height %d)", tipNow, peerStartHeight)
					}
					applog.Line("headers", "peer returned empty headers list - chain tip caught up")
					return nil
				}
				if invSinceHeaders > 0 {
					applog.Line("headers", fmt.Sprintf("peer sent %d inv message(s) (%d inv entries) before this %d-header reply", invSinceHeaders, invEntriesSinceHeaders, count))
				}
				invSinceHeaders = 0
				invEntriesSinceHeaders = 0
				tip, _ := j.TipHeight()
				lastHeadersReceived = time.Now()
				tipTime := headerTipBlockTimeUnix(j, tip)
				nowU := time.Now().Unix()
				if bs != nil {
					nowU = bs.NetworkTimeUnix()
				}
				stallLimit = headerSyncStallLimit(tip, peerStartHeight, bodiesBehind, tipTime, nowU)
				applog.Line("headers", fmt.Sprintf("received %d headers, validated & appended (tip height %d)", count, tip))
				NoteHeadersAppended(count, tip)
				if headerScorer != nil && w.PeerAddr != "" {
					headerScorer.NoteHeadersDelivered(w.PeerAddr, count)
				}
				if rawSync != nil {
					rawSync.OnTipChanged(tip)
				}
				if partial {
					applog.Line("headers", fmt.Sprintf("peer returned partial batch (%d headers, <2000) - requesting next chunk via new getheaders", count))
					// Do not return: a small batch is normal mid-IBD; only an empty headers message means we are caught up.
					continue outer
				}
				// Genesis-only during IBD: tip-window getdata runs after DownloadHeaders in Run().
				if round%5 == 0 {
					tryInterleaveGenesisOnly()
				}
				continue outer
			case "reject":
				rj, err := wire.DecodeRejectPayload(pl)
				if err != nil {
					return fmt.Errorf("reject during headers sync (malformed): %w", err)
				}
				return fmt.Errorf("reject during headers sync: %s", rj.String())
			case "feefilter":
				fee, err := wire.DecodeFeeFilterPayload(pl)
				if err != nil {
					applog.Line("headers", "feefilter (malformed): "+err.Error())
				} else {
					if feeSink != nil {
						feeSink.Set("", fee)
					}
					applog.Line("net", fmt.Sprintf("peer feefilter: min relay fee rate %d (wire koinu per kB)", fee))
				}
				continue
			case "sendcmpct":
				if _, err := ReplySendCmpctDecline(w, pl); err != nil {
					applog.Line("headers", "sendcmpct: "+err.Error())
				}
				continue
			case "addr":
				if feed != nil {
					if n := feed.NoteFromAddrPayload(pl); n > 0 {
						applog.Line("net", fmt.Sprintf("header sync: learned %d addr(s) from peer", n))
					}
				}
				continue
			default:
				if cmd == "inv" {
					entries, err := wire.DecodeInvPayload(pl)
					if err != nil {
						applog.Line("headers", fmt.Sprintf("inv payload %d bytes (decode err: %v)", len(pl), err))
						continue
					}
					invSinceHeaders++
					invEntriesSinceHeaders += len(entries)
					if !headersOnly && bodiesBehind && bs != nil {
						// Forward body IBD uses progressive getdata; inv blocks on this link cause timeouts.
						continue
					}
					if bs != nil && invBlockFetchWorthwhile(bs, entries) {
						HandleInvBlockFetchEntries(ctx, w, p, bs, entries)
					}
					continue
				}
				if isBenignHeaderSyncNoise(cmd) {
					continue
				}
				applog.Line("net", fmt.Sprintf("while waiting for headers: cmd=%q payload=%d bytes (ignored until headers/ping/reject)", cmd, len(pl)))
			}
		}
	}
	tip, _ := j.TipHeight()
	if maxRounds < 4096 {
		if peerStartHeight > 0 && int64(peerStartHeight) > tip+headerCatchUpPeerLead {
			if bodiesBehind {
				applog.Line("headers", fmt.Sprintf("deferring further headers at tip %d (peer height %d); background header catch-up required (Core IBD)", tip, peerStartHeight))
				return fmt.Errorf("header sync yielded at height %d (peer %d): background catch-up required", tip, peerStartHeight)
			}
			return fmt.Errorf("header sync incomplete at height %d (peer reports %d): connect again or wait for block catch-up before more headers", tip, peerStartHeight)
		}
		applog.Line("headers", fmt.Sprintf("header sync round limit (%d) at tip %d; block body download continues", maxRounds, tip))
	}
	return nil
}

func deadlineFromCtx(ctx context.Context, fallback time.Duration) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	return time.Now().Add(fallback)
}

// readNextP2PMessage reads one frame but returns when ctx expires (closes conn to unblock ReadMessage).
func readNextP2PMessage(ctx context.Context, conn net.Conn, magic [4]byte, batchEnd time.Time) (string, []byte, error) {
	type msgResult struct {
		cmd string
		pl  []byte
		err error
	}
	ch := make(chan msgResult, 1)
	go func() {
		_ = conn.SetReadDeadline(batchBlockReadDeadline(batchEnd))
		cmd, pl, err := wire.ReadMessage(conn, magic)
		ch <- msgResult{cmd, pl, err}
	}()
	select {
	case <-ctx.Done():
		if c, ok := conn.(interface{ Close() error }); ok {
			_ = c.Close()
		}
		return "", nil, ctx.Err()
	case r := <-ch:
		return r.cmd, r.pl, r.err
	}
}

// isBenignHeaderSyncNoise identifies P2P chatter that is normal while waiting for the next "headers"
// reply; logging each message floods the UI/debug log without helping diagnosis.
func isBenignHeaderSyncNoise(cmd string) bool {
	switch cmd {
	case "sendheaders", "addr", "getheaders":
		return true
	default:
		return false
	}
}

const (
	genesisFetchMaxReads    = 32
	genesisFetchReadTimeout = 12 * time.Second
)

// fetchAndStoreRawBlock requests one block by id, validates, and stores via BlockStoreCtx.
func fetchAndStoreRawBlock(ctx context.Context, w *MsgWriter, p chain.Params, want [32]byte, bs *BlockStoreCtx) error {
	if bs == nil || bs.Raw == nil {
		return nil
	}
	height := int64(-1)
	if bs.Journal != nil {
		if h, err := bs.Journal.HeightByBlockHashLE(want); err == nil {
			height = h
		}
	}
	if bs.rawBodyPresent(want, height) {
		return nil
	}
	return fetchAndStoreRawBlockInv(ctx, w, p, want, bs, wire.InvTypeBlock, 500, 60*time.Second)
}

func fetchAndStoreRawBlockInv(ctx context.Context, w *MsgWriter, p chain.Params, want [32]byte, bs *BlockStoreCtx, invType uint32, maxReads int, readTimeout time.Duration) error {
	if bs == nil || bs.Raw == nil {
		return nil
	}
	height := int64(-1)
	if bs.Journal != nil {
		if h, err := bs.Journal.HeightByBlockHashLE(want); err == nil {
			height = h
		}
	}
	if bs.rawBodyPresent(want, height) {
		return nil
	}
	// Allow explicit genesis fetch during forward IBD; chainActive cannot advance from -1 without height 0.
	if BodiesBehindHeaders(bs) && height != 0 {
		return fmt.Errorf("deferred during forward block IBD")
	}
	conn := w.Conn()
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: invType, Hash: want}})
	if err != nil {
		return err
	}
	if err := w.Write("getdata", pl); err != nil {
		return err
	}
	applog.Line("block", fmt.Sprintf("getdata for block %x (payload %d bytes)", want[:8], len(pl)))
	if maxReads < 1 {
		maxReads = 1
	}
	if readTimeout <= 0 {
		readTimeout = 60 * time.Second
	}
	for i := 0; i < maxReads; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if i > 0 && i%8 == 0 {
			applog.Line("block", fmt.Sprintf("still waiting for block %x… (read %d/%d on %s)", want[:4], i, maxReads, w.PeerAddr))
		}
		_ = conn.SetReadDeadline(deadlineFromCtx(ctx, readTimeout))
		cmd, payload, err := wire.ReadMessage(conn, p.Magic)
		if err != nil {
			return err
		}
		switch cmd {
		case "ping":
			if err := w.Write("pong", payload); err != nil {
				return err
			}
		case "block":
			if err := bs.StoreValidatedBlock(want, payload); err != nil {
				applog.Line("block", fmt.Sprintf("block store %x…: %v%s", want[:4], err, consensus.LegacySubsidyBugHint(err)))
				continue
			}
			notifyBlockFromPeer(bs, w.PeerAddr, want)
			return nil
		case "notfound":
			entries, err := wire.DecodeInvPayload(payload)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.Hash == want && (e.Type == wire.InvTypeBlock || e.Type == wire.InvTypeWitnessBlock) {
					return fmt.Errorf("notfound for block %x", want)
				}
			}
		case "reject":
			rj, err := wire.DecodeRejectPayload(payload)
			if err != nil {
				return fmt.Errorf("reject before block (malformed): %w", err)
			}
			return fmt.Errorf("reject before block: %s", rj.String())
		default:
			if isBenignBlockFetchNoise(cmd) {
				continue
			}
		}
	}
	return fmt.Errorf("timeout: no valid block for %x", want)
}

func isBenignBlockFetchNoise(cmd string) bool {
	switch cmd {
	case "inv", "headers", "tx", "sendheaders", "sendcmpct", "feefilter", "addr",
		"cmpctblock", "blocktxn", "getheaders", "pong":
		return true
	default:
		return false
	}
}

// fetchAndStoreRawBlocksBatch requests multiple blocks in one getdata and stores each "block" reply.
// heights, when len(heights)==len(wants), avoids O(tip) hash scans during forward IBD.
// hooks, when set, pipelines another getdata before the current request drains (ltcd minInFlightBlocks).
type getdataBatchHooks struct {
	OnStored func(height int64)
	Refill   func(pending int) (hashes [][32]byte, heights []int64)
}

// storeInboundBlockBody writes a full block if the header journal knows its height.
// Used for claimed getdata, assist drain leftovers, and late replies after refill.
func storeInboundBlockBody(bs *BlockStoreCtx, payload []byte, heightHint int64) (height int64, stored bool) {
	if bs == nil || bs.Raw == nil || len(payload) < 80 {
		return -1, false
	}
	got := pow.BlockHashLE(payload[:80])
	height = heightHint
	if height < 0 && bs.Journal != nil {
		if ht, err := bs.Journal.HeightByBlockHashLE(got); err == nil {
			height = ht
		}
	}
	if height < 0 {
		return -1, false
	}
	if bs.rawBodyPresent(got, height) {
		return height, false
	}
	if err := bs.StoreValidatedBlockAtHeight(got, payload, height); err != nil {
		return height, false
	}
	return height, true
}

func fetchAndStoreRawBlocksBatch(ctx context.Context, w *MsgWriter, p chain.Params, wants [][32]byte, heights []int64, bs *BlockStoreCtx, syncLanes int, hooks *getdataBatchHooks) (int, error) {
	hashHeights := batchHashHeights(wants, heights)
	invOrder := blockFetchInvTypes(p)
	stored := 0
	var lastErr error
	perTry := EffectiveBlockDownloadTimeout(bs, syncLanes)
	if len(invOrder) > 1 {
		perTry = perTry / time.Duration(len(invOrder))
		if perTry < 45*time.Second {
			perTry = 45 * time.Second
		}
	}
	for _, invType := range invOrder {
		still, stillHeights := wantsMissingRaw(bs, wants, heights, hashHeights)
		if len(still) == 0 {
			return stored, lastErr
		}
		tryCtx := ctx
		cancelTry := func() {}
		if hooks == nil {
			var cancel context.CancelFunc
			tryCtx, cancel = context.WithTimeout(ctx, perTry)
			cancelTry = cancel
		}
		n, err := fetchAndStoreRawBlocksBatchInv(tryCtx, w, p, still, batchHashHeights(still, stillHeights), bs, invType, syncLanes, hooks)
		cancelTry()
		stored += n
		if err != nil {
			lastErr = err
		}
	}
	return stored, lastErr
}

func applyGetDataRefill(w *MsgWriter, pending map[[32]byte]struct{}, hashHeights map[[32]byte]int64, invType uint32, hooks *getdataBatchHooks) (int, error) {
	if w == nil || hooks == nil || hooks.Refill == nil || pending == nil || !shouldRefillGetData(len(pending)) {
		return 0, nil
	}
	hashes, heights := hooks.Refill(len(pending))
	if len(hashes) == 0 {
		return 0, nil
	}
	fresh := make([]wire.InvEntry, 0, len(hashes))
	for i, h := range hashes {
		if _, already := pending[h]; already {
			continue
		}
		ht := int64(-1)
		if i < len(heights) {
			ht = heights[i]
		}
		if hashHeights != nil {
			hashHeights[h] = ht
		}
		pending[h] = struct{}{}
		fresh = append(fresh, wire.InvEntry{Type: invType, Hash: h})
	}
	if len(fresh) == 0 {
		return 0, nil
	}
	pl, err := wire.EncodeGetData(fresh)
	if err != nil {
		return 0, err
	}
	if err := w.Write("getdata", pl); err != nil {
		return 0, err
	}
	peer := w.PeerAddr
	applog.Line("block", fmt.Sprintf("getdata refill +%d block(s) on %s (%d in flight)", len(fresh), peer, len(pending)))
	return len(fresh), nil
}

func wantsMissingRaw(bs *BlockStoreCtx, wants [][32]byte, heights []int64, hashHeights map[[32]byte]int64) (still [][32]byte, stillHeights []int64) {
	for i, h := range wants {
		height := hashHeights[h]
		if height < 0 && heights != nil && i < len(heights) {
			height = heights[i]
		}
		if height < 0 && bs.Journal != nil {
			if ht, err := bs.Journal.HeightByBlockHashLE(h); err == nil {
				height = ht
			}
		}
		if bs.Raw == nil || !bs.rawBodyPresent(h, height) {
			still = append(still, h)
			stillHeights = append(stillHeights, height)
		}
	}
	return still, stillHeights
}

func batchHashHeights(wants [][32]byte, heights []int64) map[[32]byte]int64 {
	m := make(map[[32]byte]int64, len(wants))
	for i, h := range wants {
		if heights != nil && i < len(heights) && heights[i] >= 0 {
			m[h] = heights[i]
		}
	}
	return m
}

func fetchAndStoreRawBlocksBatchInv(ctx context.Context, w *MsgWriter, p chain.Params, wants [][32]byte, hashHeights map[[32]byte]int64, bs *BlockStoreCtx, invType uint32, syncLanes int, hooks *getdataBatchHooks) (int, error) {
	if bs == nil || bs.Raw == nil || len(wants) == 0 {
		return 0, nil
	}
	pending := make(map[[32]byte]struct{}, len(wants))
	entries := make([]wire.InvEntry, 0, len(wants))
	for _, h := range wants {
		height := hashHeights[h]
		if height < 0 && bs.Journal != nil {
			if ht, err := bs.Journal.HeightByBlockHashLE(h); err == nil {
				height = ht
			}
		}
		if bs.rawBodyPresent(h, height) {
			continue
		}
		pending[h] = struct{}{}
		entries = append(entries, wire.InvEntry{Type: invType, Hash: h})
	}
	if len(entries) == 0 {
		return 0, nil
	}
	conn := w.Conn()
	pl, err := wire.EncodeGetData(entries)
	if err != nil {
		return 0, err
	}
	if err := w.Write("getdata", pl); err != nil {
		return 0, err
	}
	if invType == wire.InvTypeWitnessBlock {
		applog.Line("block", fmt.Sprintf("batched getdata (MSG_WITNESS_BLOCK) for %d block(s) on %s", len(entries), w.PeerAddr))
	} else {
		applog.Line("block", fmt.Sprintf("batched getdata (MSG_BLOCK) for %d block(s) on %s", len(entries), w.PeerAddr))
	}
	stored := 0
	stubRejects := 0
	requested := len(entries)
	var ignoredCmds []string
	maxReads := 500 * requested
	enqueueRefill := func() error {
		added, err := applyGetDataRefill(w, pending, hashHeights, invType, hooks)
		if err != nil {
			return err
		}
		if added > 0 {
			requested += added
			maxReads += 500 * added
		}
		return nil
	}
	if err := enqueueRefill(); err != nil {
		return 0, err
	}
	batchDL := blockBatchDeadline(ctx, syncLanes, bs)
	abortBatch := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if c, ok := conn.(interface{ Close() error }); ok {
				_ = c.Close()
			}
		case <-abortBatch:
		}
	}()
	hardTimer := time.AfterFunc(time.Until(batchDL)+250*time.Millisecond, func() {
		if c, ok := conn.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	})
	defer func() {
		close(abortBatch)
		hardTimer.Stop()
	}()
	for i := 0; i < maxReads && len(pending) > 0; i++ {
		select {
		case <-ctx.Done():
			return stored, ctx.Err()
		default:
		}
		if time.Now().After(batchDL) {
			if len(pending) > 0 {
				applog.Line("block", fmt.Sprintf("getdata batch deadline expired with %d/%d block(s) still pending on %s", len(pending), requested, w.PeerAddr))
			}
			break
		}
		_ = conn.SetReadDeadline(batchBlockReadDeadline(batchDL))
		cmd, payload, err := readNextP2PMessage(ctx, conn, p.Magic, batchDL)
		if err != nil {
			return stored, err
		}
		switch cmd {
		case "ping":
			if err := w.Write("pong", payload); err != nil {
				return stored, err
			}
		case "block":
			if len(payload) < 80 {
				continue
			}
			got := pow.BlockHashLE(payload[:80])
			if _, ok := pending[got]; !ok {
				hint := int64(-1)
				if hashHeights != nil {
					if h, ok := hashHeights[got]; ok {
						hint = h
					}
				}
				if ht, storedLate := storeInboundBlockBody(bs, payload, hint); storedLate {
					stored++
					if hooks != nil && hooks.OnStored != nil && ht >= 0 {
						hooks.OnStored(ht)
					}
					continue
				}
				if len(ignoredCmds) < 4 {
					ignoredCmds = append(ignoredCmds, fmt.Sprintf("block %x… (%d B)", got[:4], len(payload)))
				}
				continue
			}
			height := hashHeights[got]
			if height < 0 {
				applog.Line("block", fmt.Sprintf("batched block %x… (%d B): no claim height; skipping", got[:4], len(payload)))
				delete(pending, got)
				continue
			}
			var err error
			if height >= 0 {
				err = bs.StoreValidatedBlockAtHeight(got, payload, height)
			} else {
				err = bs.StoreValidatedBlock(got, payload)
			}
			if err != nil {
				applog.Line("block", fmt.Sprintf("batched block store %x…: %v%s", got[:4], err, consensus.LegacySubsidyBugHint(err)))
				if strings.Contains(err.Error(), "too short") {
					stubRejects++
					delete(pending, got)
				}
				continue
			}
			notifyBlockFromPeer(bs, w.PeerAddr, got)
			delete(pending, got)
			stored++
			if hooks != nil && hooks.OnStored != nil {
				hooks.OnStored(height)
			}
			// Core MarkBlockAsReceived: nDownloadingSince = now when the front of the
			// in-flight queue is delivered. Refresh the batch window so slow-but-live
			// ancient getdata is not killed after the first block.
			limit := EffectiveBlockDownloadTimeout(bs, syncLanes)
			batchDL = time.Now().Add(limit)
			hardTimer.Reset(limit + 250*time.Millisecond)
			if stored == 1 && requested <= 4 {
				applog.Line("block", fmt.Sprintf("batched block %x… stored (%d B) height %d", got[:4], len(payload), height))
			}
			if err := enqueueRefill(); err != nil {
				return stored, err
			}
		case "notfound":
			nf, err := wire.DecodeInvPayload(payload)
			if err != nil {
				continue
			}
			for _, e := range nf {
				if e.Type == invType || e.Type == wire.InvTypeBlock || e.Type == wire.InvTypeWitnessBlock {
					delete(pending, e.Hash)
				}
			}
			if err := enqueueRefill(); err != nil {
				return stored, err
			}
		case "reject":
			rj, err := wire.DecodeRejectPayload(payload)
			if err != nil {
				return stored, fmt.Errorf("reject before block (malformed): %w", err)
			}
			return stored, fmt.Errorf("reject before block: %s", rj.String())
		default:
			if isBenignBlockFetchNoise(cmd) {
				continue
			}
			if len(ignoredCmds) < 6 {
				ignoredCmds = append(ignoredCmds, cmd)
			}
		}
	}
	if stored == 0 && requested > 0 {
		msg := fmt.Sprintf("batch incomplete: 0/%d block(s) stored", requested)
		if len(pending) > 0 {
			msg = fmt.Sprintf("batch incomplete: %d/%d block(s) missing (notfound or timeout)", len(pending), requested)
		}
		if stubRejects > 0 {
			msg += fmt.Sprintf("; rejected %d undersized stub(s)", stubRejects)
		}
		if len(ignoredCmds) > 0 {
			msg += "; also saw: " + strings.Join(ignoredCmds, ", ")
		}
		return stored, fmt.Errorf("%s", msg)
	}
	return stored, nil
}

// maxInvBlockFetchPerMessage limits getdata traffic when reacting to inv bursts.
const maxInvBlockFetchPerMessage = 16

// HandleInvBlockFetch reacts to block inventory: requests missing blocks via getdata (best-effort).
func HandleInvBlockFetch(ctx context.Context, w *MsgWriter, p chain.Params, bs *BlockStoreCtx, invPayload []byte) {
	if bs == nil || bs.Raw == nil {
		return
	}
	entries, err := wire.DecodeInvPayload(invPayload)
	if err != nil {
		return
	}
	HandleInvBlockFetchEntries(ctx, w, p, bs, entries)
}

// HandleInvBlockFetchEntries schedules getdata for missing block inv vectors (capped per message).
func HandleInvBlockFetchEntries(ctx context.Context, w *MsgWriter, p chain.Params, bs *BlockStoreCtx, entries []wire.InvEntry) {
	if bs == nil || bs.Raw == nil || len(entries) == 0 {
		return
	}
	var nBlk, nTx, nOther int
	for _, e := range entries {
		switch e.Type {
		case wire.InvTypeBlock, wire.InvTypeWitnessBlock:
			nBlk++
		case wire.InvTypeTx, wire.InvTypeWitnessTx:
			nTx++
		default:
			nOther++
		}
	}
	if nBlk > 0 {
		applog.Line("block", fmt.Sprintf("inv: %d block(s), %d tx inv(s), %d other; scheduling getdata for blocks (capped)", nBlk, nTx, nOther))
	} else if nTx > 0 && !ShouldSuppressInvTxFetchDuringIBD(bs) {
		applog.Line("mempool", fmt.Sprintf("inv: %d tx advertisement(s); getdata handled in steady-state loop (capped)", nTx))
	}
	done := 0
	for _, e := range entries {
		if done >= maxInvBlockFetchPerMessage {
			return
		}
		switch e.Type {
		case wire.InvTypeBlock, wire.InvTypeWitnessBlock:
			height := int64(-1)
			if bs.Journal != nil {
				if h, err := bs.Journal.HeightByBlockHashLE(e.Hash); err == nil {
					height = h
				}
			}
			if bs.rawBodyPresent(e.Hash, height) {
				continue
			}
			if ShouldDeferInvBlockFetch(bs, e.Hash) {
				continue
			}
			done++
			if err := fetchAndStoreRawBlockInv(ctx, w, p, e.Hash, bs, e.Type, 500, 60*time.Second); err != nil {
				if ctx.Err() != nil || IsBenignShutdownErr(err) {
					return
				}
				if strings.Contains(err.Error(), "deferred during forward block IBD") {
					continue
				}
				applog.Line("block", fmt.Sprintf("inv block fetch %x: %v", e.Hash[:8], err))
			}
		default:
		}
	}
}

// SyncGenesisRawBlock ensures genesis is stored (chainparams first, then optional P2P getdata fallback).
func SyncGenesisRawBlock(ctx context.Context, w *MsgWriter, p chain.Params, bs *BlockStoreCtx) error {
	if bs == nil || bs.Raw == nil || bs.Journal == nil {
		return nil
	}
	if err := EnsureLocalGenesis(bs); err != nil {
		applog.Line("block", "local genesis: "+err.Error())
	}
	if !NeedsGenesisBlock(bs) {
		return nil
	}
	if w == nil {
		return fmt.Errorf("genesis block missing and no peer connection for getdata fallback")
	}
	if store.HasStoredBodyAtHeight(bs.Journal, bs.Raw, 0, bs.chainNet()) {
		return nil
	}
	h0, err := bs.Journal.ReadHeaderAt(0)
	if err != nil {
		return err
	}
	want := pow.BlockHashLE(h0)
	errB := fetchAndStoreRawBlockInv(ctx, w, p, want, bs, wire.InvTypeBlock, genesisFetchMaxReads, genesisFetchReadTimeout)
	if errB == nil {
		if store.HasStoredBodyAtHeight(bs.Journal, bs.Raw, 0, bs.chainNet()) {
			return nil
		}
		return fmt.Errorf("genesis fetch: stored payload too small for mainnet")
	}
	if strings.Contains(strings.ToLower(errB.Error()), "notfound") {
		errB = fmt.Errorf("%w: %v", ErrGenesisPeerNotFound, errB)
	}
	if isPermanentFetchErr(errB) {
		return errB
	}
	errW := fetchAndStoreRawBlockInv(ctx, w, p, want, bs, wire.InvTypeWitnessBlock, genesisFetchMaxReads, genesisFetchReadTimeout)
	if errW == nil {
		if store.HasStoredBodyAtHeight(bs.Journal, bs.Raw, 0, bs.chainNet()) {
			return nil
		}
		return fmt.Errorf("genesis fetch: stored payload too small for mainnet")
	}
	if strings.Contains(strings.ToLower(errW.Error()), "notfound") {
		return fmt.Errorf("%w: %v", ErrGenesisPeerNotFound, errW)
	}
	if isPermanentFetchErr(errW) {
		return errW
	}
	return errB
}

// tipBackfillRange returns the lowest height to fetch for a tip-aligned batch (inclusive), or ok=false if none.
func tipBackfillRange(tip int64, maxHeights int) (start int64, ok bool) {
	if tip < 1 || maxHeights <= 0 {
		return 0, false
	}
	start = tip - int64(maxHeights) + 1
	if start < 1 {
		start = 1
	}
	return start, true
}

// SyncRecentRawBlocks requests full blocks for the highest maxHeights headers (heights start..tip),
// where start = max(1, tip-maxHeights+1). Height 0 is left to SyncGenesisRawBlock. Best-effort per height.
func SyncRecentRawBlocks(ctx context.Context, w *MsgWriter, p chain.Params, bs *BlockStoreCtx, maxHeights int) {
	if bs == nil || bs.Raw == nil || bs.Journal == nil || maxHeights <= 0 {
		return
	}
	j := bs.Journal
	tip, err := j.TipHeight()
	if err != nil {
		fmt.Fprintf(os.Stderr, "raw blocks batch: tip: %v\n", err)
		return
	}
	if tip < 1 {
		return
	}
	start, ok := tipBackfillRange(tip, maxHeights)
	if !ok {
		return
	}
	applog.Line("block", fmt.Sprintf("raw block tip backfill heights %d..%d (tip=%d)", start, tip, tip))
	for h := start; h <= tip; h++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			fmt.Fprintf(os.Stderr, "raw blocks batch: read header %d: %v\n", h, err)
			return
		}
		want := pow.BlockHashLE(h80)
		if err := fetchAndStoreRawBlock(ctx, w, p, want, bs); err != nil {
			if isPermanentFetchErr(err) {
				fmt.Fprintf(os.Stderr, "raw blocks batch: peer closed connection at height %d (%v); stopping batch\n", h, err)
				return
			}
			fmt.Fprintf(os.Stderr, "raw blocks batch: height %d: %v\n", h, err)
		}
		if h < tip && rawBlockTipFetchPace > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(rawBlockTipFetchPace):
			}
		}
	}
}

func notifyBlockFromPeer(bs *BlockStoreCtx, peerAddr string, want [32]byte) {
	if bs == nil || bs.OnBlockFromPeer == nil || peerAddr == "" {
		return
	}
	height := int64(-1)
	if bs.Journal != nil {
		if h, err := bs.Journal.HeightByBlockHashLE(want); err == nil {
			height = h
		}
	}
	bs.OnBlockFromPeer(peerAddr, height)
}
