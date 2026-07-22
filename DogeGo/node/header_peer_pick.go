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
	"sort"
	"sync"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/p2p"
	"dogego/wire"
)

const (
	headerSyncPeerProbeMax          = 6  // handshaked candidates for header-sync failover
	headerSyncPeerProbeDials        = 48 // max TCP attempts before starting with whoever connected
	headerSyncPeerProbeConcurrency  = 8  // parallel dials during startup probe
	headerSyncProbeFreshCap         = 80 // prefer DNS/fixed seeds before walking entire score file
)

// headerSyncPeer is a handshaked outbound peer candidate for headers-first sync.
type headerSyncPeer struct {
	addr string
	conn net.Conn
	mw   *MsgWriter
	ctr  *netByteCounter
	dv   *wire.DecodedVersion
}

func (p headerSyncPeer) startHeight() int32 {
	if p.dv == nil {
		return 0
	}
	return p.dv.StartHeight
}

func closeHeaderSyncPeer(p headerSyncPeer) {
	if p.conn != nil {
		_ = p.conn.Close()
	}
}

// dialExtraHeaderSyncPeer handshakes one outbound peer excluding excludeAddr (for a block-primary
// session while header sync runs on another connection).
func dialExtraHeaderSyncPeer(ctx context.Context, d net.Dialer, addrs []string, p chain.Params, subVer string, localServices uint64, excludeAddr string, scorer *BlockPeerScorer, book *AddrBook, wantBlockHeight int64) (headerSyncPeer, error) {
	if scorer != nil && wantBlockHeight >= 0 {
		addrs = scorer.DialableOrderForBlock(addrs, excludeAddr, wantBlockHeight)
	}
	for _, addr := range addrs {
		if addr == "" || addr == excludeAddr {
			continue
		}
		peer, err := probeDialHandshake(ctx, d, addr, p, subVer, localServices, scorer, book)
		if err != nil {
			if scorer != nil {
				scorer.NoteDialFailure(addr)
			}
			continue
		}
		return peer, nil
	}
	return headerSyncPeer{}, fmt.Errorf("no handshaked peer excluding %q", excludeAddr)
}

// HeaderSyncProbeCandidates orders dial targets for startup: addnode first, then DNS/seeds, then score history.
func HeaderSyncProbeCandidates(discovered []string, scorer *BlockPeerScorer, addnode []string) []string {
	seen := make(map[string]struct{})
	var manual []string
	addManual := func(a string) {
		if a == "" {
			return
		}
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		manual = append(manual, a)
	}
	for _, a := range addnode {
		addManual(a)
	}
	var fresh []string
	addFresh := func(a string) {
		if a == "" {
			return
		}
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		fresh = append(fresh, a)
	}
	for i, a := range discovered {
		if i >= headerSyncProbeFreshCap {
			break
		}
		addFresh(a)
	}
	var rest []string
	addRest := func(a string) {
		if a == "" {
			return
		}
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		rest = append(rest, a)
	}
	if scorer != nil {
		for _, row := range scorer.TopPeers(48) {
			addRest(row.Addr)
		}
		for _, a := range scorer.KnownAddresses() {
			addRest(a)
		}
	}
	for i, a := range discovered {
		if i < headerSyncProbeFreshCap {
			continue
		}
		addRest(a)
	}
	if scorer != nil {
		rest = scorer.DialableOrder(rest, "")
	}
	out := make([]string, 0, len(manual)+len(fresh)+len(rest))
	out = append(out, manual...)
	// /16 spread so parallel header probes hit diverse networks (DNS seeds often cluster).
	out = append(out, SpreadHostPortsByGroup16(fresh)...)
	out = append(out, SpreadHostPortsByGroup16(rest)...)
	return p2p.PreferIPv4First(out)
}

// probeHeaderSyncPeers handshakes up to maxProbe discovery candidates and returns them sorted by
// advertised start height descending (Core: prefer peers on the longest chain).
func probeHeaderSyncPeers(ctx context.Context, d net.Dialer, addrs []string, p chain.Params, subVer string, localServices uint64, maxProbe int, scorer *BlockPeerScorer, book *AddrBook) ([]headerSyncPeer, error) {
	if maxProbe < 1 {
		maxProbe = 1
	}
	if maxProbe > headerSyncPeerProbeMax {
		maxProbe = headerSyncPeerProbeMax
	}
	probeDialer := d
	probeDialer.Timeout = 8 * time.Second
	workers := headerSyncPeerProbeConcurrency
	if workers > maxProbe {
		workers = maxProbe
	}
	applog.Line("net", fmt.Sprintf("header sync peer probe: up to %d handshakes, %d parallel workers (max %d dials, %s timeout)",
		maxProbe, workers, headerSyncPeerProbeDials, probeDialer.Timeout))

	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var out []headerSyncPeer
	var lastDialErr error
	var addrIdx int
	var attempts int

	tryNextAddr := func() (string, bool) {
		mu.Lock()
		defer mu.Unlock()
		if len(out) >= maxProbe || attempts >= headerSyncPeerProbeDials {
			return "", false
		}
		i := addrIdx
		addrIdx++
		if i >= len(addrs) {
			return "", false
		}
		attempts++
		return addrs[i], true
	}

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-probeCtx.Done():
					return
				default:
				}
				mu.Lock()
				done := len(out) >= maxProbe || attempts >= headerSyncPeerProbeDials
				mu.Unlock()
				if done {
					return
				}
				addr, ok := tryNextAddr()
				if !ok {
					return
				}
				peer, err := probeDialHandshake(probeCtx, probeDialer, addr, p, subVer, localServices, scorer, book)
				if err != nil {
					mu.Lock()
					lastDialErr = err
					n := int(attempts)
					connected := len(out)
					mu.Unlock()
					msg := fmt.Sprintf("dial %s: %v (probe %d/%d, %d connected)", addr, err, n, headerSyncPeerProbeDials, connected)
					if isBadMagicP2PErr(err) {
						msg += fmt.Sprintf(" - expected P2P magic %x for this network", p.Magic)
					}
					applog.Line("net", msg)
					continue
				}
				mu.Lock()
				if len(out) < maxProbe {
					out = append(out, peer)
					applog.Line("net", fmt.Sprintf("probed %s (peer start height %d)", addr, peer.startHeight()))
					if len(out) >= maxProbe {
						cancel()
					}
				} else {
					closeHeaderSyncPeer(peer)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	nAttempts := attempts
	mu.Unlock()
	if len(out) == 0 {
		if lastDialErr != nil {
			return nil, fmt.Errorf("no peer handshakes succeeded after %d dial attempts (last: %w); set peer=host:port in dogecoinconf.json or check firewall", nAttempts, lastDialErr)
		}
		return nil, fmt.Errorf("no peer handshakes succeeded after %d dial attempts", nAttempts)
	}
	if len(out) < maxProbe {
		applog.Line("net", fmt.Sprintf("header sync peer probe: starting with %d peer(s) after %d dial attempt(s)", len(out), nAttempts))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].startHeight() > out[j].startHeight()
	})
	applog.Line("headers", fmt.Sprintf("header sync peer probe: %d handshaked, best %s (start height %d)",
		len(out), out[0].addr, out[0].startHeight()))
	return out, nil
}

// pickBlockPrimaryPeer chooses the best outbound peer for block getdata among probed sessions.
// Prefers full NODE_NETWORK peers when wantHeight is ancient (Core: pruned peers cannot serve genesis).
func pickBlockPrimaryPeer(probed []headerSyncPeer, excludeHeaderAddr string, wantHeight int64) (headerSyncPeer, bool) {
	bestScore := -1 << 30
	var best headerSyncPeer
	found := false
	for _, p := range probed {
		if p.addr == "" || p.addr == excludeHeaderAddr || p.dv == nil {
			continue
		}
		score := int(p.startHeight())
		if chain.PeerLikelyHasBlock(p.dv.Services, p.dv.StartHeight, wantHeight) {
			if chain.HasFullBlockRelay(p.dv.Services) && p.dv.Services&chain.ServiceNetworkLimited == 0 {
				score += 1_000_000_000
			} else {
				score += 100_000_000
			}
		} else {
			score -= 500_000_000
		}
		if score > bestScore {
			bestScore = score
			best = p
			found = true
		}
	}
	return best, found
}

func probeDialHandshake(ctx context.Context, d net.Dialer, addr string, p chain.Params, subVer string, localServices uint64, scorer *BlockPeerScorer, book *AddrBook) (headerSyncPeer, error) {
	RecordOutboundDialTry(book, addr)
	c, _, err := DialP2POutbound(ctx, d, addr)
	if err != nil {
		RecordOutboundHandshakeResult(book, addr, err)
		if scorer != nil {
			scorer.NoteDialFailure(addr)
		}
		return headerSyncPeer{}, err
	}
	ctr := newNetByteCounter()
	wrapped := &countingConn{Conn: c, ctr: ctr}
	dv, err := Handshake(ctx, wrapped, p, subVer, localServices)
	if err != nil {
		_ = wrapped.Close()
		RecordOutboundHandshakeResult(book, addr, err)
		if scorer != nil {
			scorer.NoteDialFailure(addr)
		}
		return headerSyncPeer{}, err
	}
	RecordOutboundPeerHandshake(book, scorer, addr, dv, nil)
	return headerSyncPeer{
		addr: addr,
		conn: wrapped,
		mw:   NewMsgWriter(wrapped, p.Magic),
		ctr:  ctr,
		dv:   dv,
	}, nil
}
