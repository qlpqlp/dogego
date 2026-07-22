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
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/wire"
)

const (
	maxPrimaryRedialTries    = 12
	primaryRedialMinInterval = 3 * time.Second
	maxPrimaryRedialStreak   = 24
)

// PrimaryRedialOpts configures primary peer reconnection after a dropped session.
type PrimaryRedialOpts struct {
	Ctx         context.Context
	Dialer      net.Dialer
	Params      chain.Params
	UserAgent      string
	LocalServices  uint64 // NODE_* on version (0 → Params.NodeNetwork)
	FixedPeer      string // non-empty: only redial this host (-connect style)
	Discovered  []string
	ExcludeAddr     string
	Scorer          *BlockPeerScorer
	PeerMgr         *PeerMgr
	AddrBook        *AddrBook
	WantBlockHeight int64 // >=0 prefer archival NODE_NETWORK peers (early block IBD)
}

// PrimaryRedialResult is a new primary TCP session after RedialPrimary succeeds.
type PrimaryRedialResult struct {
	Addr string
	Conn net.Conn
	MW   *MsgWriter
	Ctr  *netByteCounter
	DV   *wire.DecodedVersion
}

// RedialPrimary connects and handshakes with a new primary peer (Core-style reconnect during IBD).
func RedialPrimary(opts PrimaryRedialOpts) (*PrimaryRedialResult, error) {
	if opts.Ctx.Err() != nil {
		return nil, opts.Ctx.Err()
	}
	candidates := primaryRedialCandidates(opts)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no primary redial candidates")
	}
	var lastErr error
	tries := maxPrimaryRedialTries
	if len(candidates) < tries {
		tries = len(candidates)
	}
	for i := 0; i < tries; i++ {
		addr := candidates[i]
		if opts.Ctx.Err() != nil {
			return nil, opts.Ctx.Err()
		}
		RecordOutboundDialTry(opts.AddrBook, addr)
		c, _, err := DialP2POutbound(opts.Ctx, opts.Dialer, addr)
		if err != nil {
			lastErr = err
			RecordOutboundHandshakeResult(opts.AddrBook, addr, err)
			if opts.Scorer != nil {
				opts.Scorer.NoteDialFailure(addr)
			}
			continue
		}
		ctr := newNetByteCounter()
		wrapped := &countingConn{Conn: c, ctr: ctr}
		dv, err := Handshake(opts.Ctx, wrapped, opts.Params, opts.UserAgent, opts.LocalServices)
		if err != nil {
			_ = wrapped.Close()
			lastErr = err
			RecordOutboundHandshakeResult(opts.AddrBook, addr, err)
			if opts.Scorer != nil {
				opts.Scorer.NoteDialFailure(addr)
			}
			continue
		}
		RecordOutboundPeerHandshake(opts.AddrBook, opts.Scorer, addr, dv, nil)
		mw := NewMsgWriter(wrapped, opts.Params.Magic)
		mw.PeerAddr = addr
		AttachWriterMsgStats(mw)
		applog.Line("net", fmt.Sprintf("primary reconnected to %s", addr))
		return &PrimaryRedialResult{
			Addr: addr, Conn: wrapped, MW: mw, Ctr: ctr, DV: dv,
		}, nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("primary redial failed: %w", lastErr)
	}
	return nil, fmt.Errorf("primary redial failed")
}

func primaryRedialCandidates(opts PrimaryRedialOpts) []string {
	discovered := opts.Discovered
	if opts.Scorer != nil && len(discovered) > 0 {
		discovered = opts.Scorer.MergeDiscoveryCandidates(discovered, opts.WantBlockHeight)
	}
	var base []string
	seen := make(map[string]struct{})
	excludeForScorer := opts.ExcludeAddr
	if excludeForScorer == opts.FixedPeer {
		excludeForScorer = ""
	}
	add := func(a string) {
		if a == "" {
			return
		}
		if a == opts.ExcludeAddr && a != opts.FixedPeer {
			return
		}
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		base = append(base, a)
	}
	if opts.FixedPeer != "" {
		add(opts.FixedPeer)
	}
	var tail []string
	addTail := func(a string) {
		if a == "" {
			return
		}
		if a == opts.ExcludeAddr && a != opts.FixedPeer {
			return
		}
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		tail = append(tail, a)
	}
	for _, a := range discovered {
		addTail(a)
	}
	if opts.PeerMgr != nil {
		for _, a := range opts.PeerMgr.AddrPoolSnapshot() {
			addTail(a)
		}
	}
	base = append(base, SpreadHostPortsByGroup16(tail)...)
	if opts.Scorer != nil {
		if opts.WantBlockHeight >= 0 {
			return opts.Scorer.DialableOrderForBlock(base, excludeForScorer, opts.WantBlockHeight)
		}
		return opts.Scorer.DialableOrder(base, excludeForScorer)
	}
	return base
}

// ReplacePrimary swaps the main sync peer entry in PeerMgr (relay peers unchanged).
func (pm *PeerMgr) ReplacePrimary(oldAddr, newAddr string, conn net.Conn, mw *MsgWriter, ctr *netByteCounter, dv *wire.DecodedVersion) {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if l, ok := pm.sessions[oldAddr]; ok && l.primary {
		delete(pm.sessions, oldAddr)
		if l.conn != nil && l.conn != conn {
			_ = l.conn.Close()
		}
	}
	pm.primary = newAddr
	link := &peerLink{
		id: 1, addr: newAddr, conn: conn, mw: mw, ctr: ctr, peer: dv,
		since: time.Now(), primary: true,
	}
	if dv != nil {
		link.timeOffset = wire.TimeOffsetSeconds(dv, time.Now().Unix())
	}
	initPeerSyncFromVersion(link, dv)
	if mw != nil {
		mw.PeerAddr = newAddr
	}
	attachPeerMsgStats(link, mw)
	link.grantAddrTokens(maxAddrToSend)
	pm.sessions[newAddr] = link
	var order []string
	order = append(order, newAddr)
	for _, a := range pm.order {
		if a != oldAddr && a != newAddr {
			order = append(order, a)
		}
	}
	pm.order = order
	if newAddr != "" && pm.addrs != nil {
		pm.addrs.NoteSuccess(newAddr)
	}
}
