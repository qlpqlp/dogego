// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"net"
	"strings"
	"time"

	"dogego/chain"
	"dogego/rpc"
	"dogego/store"
)

func peerLocalAddr(conn net.Conn) string {
	if conn == nil {
		return ""
	}
	la := conn.LocalAddr()
	if la == nil {
		return ""
	}
	if tcp, ok := la.(*net.TCPAddr); ok {
		return net.JoinHostPort(tcp.IP.String(), fmt.Sprintf("%d", tcp.Port))
	}
	return la.String()
}

// PeerInfoContext supplies live sync and IBD metadata for getpeerinfo.
type PeerInfoContext struct {
	SyncedBlocks int64 // chainActive height (-1 if unknown)
	Scorer       *BlockPeerScorer
	Assist       *AssistPeerRegistry
	AddedNodes   *AddedNodeStore
	RawFill      *progressiveRawState
	PrimaryAddr  string
	IBDLanes     int // parallel sync lanes when >0
	Misbehavior  *MisbehaviorTracker
}

func peerInfoExtras(addr string, ctx *PeerInfoContext) map[string]interface{} {
	if ctx == nil {
		return nil
	}
	extras := make(map[string]interface{})
	if ctx.Scorer != nil {
		if st, ok := ctx.Scorer.Stats(addr); ok {
			extras["dogego_block_score"] = st.Score
			extras["dogego_blocks_delivered"] = st.Blocks
			extras["dogego_block_cooldown"] = st.InCooldown
		}
	}
	if ctx.Misbehavior != nil {
		if s := ctx.Misbehavior.Score(addr); s > 0 {
			extras["dogego_misbehavior_score"] = s
		}
	}
	return extras
}

func assistSnapshots(ctx *PeerInfoContext) []AssistPeerSnapshot {
	if ctx == nil || ctx.Assist == nil {
		return nil
	}
	return ctx.Assist.Snapshot()
}

func peerInflightRow(ctx *PeerInfoContext, addr string, primary bool) []interface{} {
	if ctx == nil || ctx.RawFill == nil {
		return []interface{}{}
	}
	prim := ctx.PrimaryAddr
	if primary && addr != "" {
		prim = addr
	}
	lane := syncLaneForPeer(addr, prim, assistSnapshots(ctx), ctx.RawFill)
	return inflightHeightsJSON(ctx.RawFill, lane)
}

func peerInflightFromSnap(byLane map[int][]int64, lane int) []interface{} {
	if lane < 0 || byLane == nil {
		return []interface{}{}
	}
	return heightsToJSONCap(byLane[lane], maxPeerInfoInflightHeights)
}

// PeerInfoMaps builds getpeerinfo-style rows for all sessions plus active block-assist workers.
func (pm *PeerMgr) PeerInfoMaps(j *store.HeaderJournal, p chain.Params, ctx *PeerInfoContext) []map[string]interface{} {
	if pm == nil {
		return assistPeerInfoRows(ctx, p, j, nil)
	}
	var tipH int64
	if j != nil {
		tipH, _ = j.TipHeight()
	}
	syncedBlocks := tipH
	if ctx != nil && ctx.SyncedBlocks >= 0 {
		syncedBlocks = ctx.SyncedBlocks
	}
	now := time.Now().Unix()
	assist := assistSnapshots(ctx)
	var byLane map[int][]int64
	if ctx != nil && ctx.RawFill != nil {
		byLane = ctx.RawFill.InflightHeightsByLane()
	}
	// Copy session pointers under the peer lock, then build rows without holding it
	// so IBD getdata can take RawFill.mu without stacking behind peer-manager work.
	pm.mu.Lock()
	links := make([]*peerLink, 0, len(pm.order))
	for _, addr := range pm.order {
		if l, ok := pm.sessions[addr]; ok {
			links = append(links, l)
		}
	}
	pm.mu.Unlock()
	out := make([]map[string]interface{}, 0, len(links)+4)
	for _, l := range links {
		row := peerInfoRow(l, p, tipH, syncedBlocks, now, ctx, pm, assist, byLane)
		out = append(out, row)
	}
	out = append(out, assistPeerInfoRowsWithSnap(ctx, p, j, out, assist, byLane)...)
	return out
}

func peerInfoRow(l *peerLink, p chain.Params, tipH int64, syncedBlocks int64, now int64, ctx *PeerInfoContext, pm *PeerMgr, assist []AssistPeerSnapshot, byLane map[int][]int64) map[string]interface{} {
	var recv, sent int64
	if l.ctr != nil {
		recv = int64(l.ctr.Recv())
		sent = int64(l.ctr.Sent())
	}
	proto := p.ProtocolVersion
	sub := ""
	svHex := rpc.FormatServicesHex(p.NodeNetwork)
	startH := int64(0)
	if l.peer != nil {
		proto = l.peer.ProtocolVersion
		sub = l.peer.UserAgent
		svHex = rpc.FormatServicesHex(l.peer.Services)
		startH = int64(l.peer.StartHeight)
	}
	connType := "outbound-full-relay"
	if l.inbound {
		connType = "inbound-full-relay"
	}
	if l.primary {
		connType = "outbound-full-relay"
	}
	note := "outbound relay peer"
	if l.primary {
		note = "primary sync peer"
	} else if l.inbound {
		note = "inbound relay peer"
	}
	lastRecv := now
	if !l.lastRecv.IsZero() {
		lastRecv = l.lastRecv.Unix()
	}
	lastSend := now
	if !l.lastSend.IsZero() {
		lastSend = l.lastSend.Unix()
	}
	lastBlock := int64(0)
	if !l.lastBlockRecv.IsZero() {
		lastBlock = l.lastBlockRecv.Unix()
	}
	lastTx := int64(0)
	if !l.lastTxRecv.IsZero() {
		lastTx = l.lastTxRecv.Unix()
	}
	relayTxes := true
	if l.peer != nil {
		relayTxes = l.peer.RelayTxes
	}
	row := map[string]interface{}{
		"id":               l.id,
		"addr":             l.addr,
		"relaytxes":        relayTxes,
		"relayaddresses":   true,
		"services":         svHex,
		"lastsend":         lastSend,
		"lastrecv":         lastRecv,
		"bytessent":        sent,
		"bytesrecv":        recv,
		"conntime":         l.since.Unix(),
		"timeoffset": l.timeOffset,
		"pingtime":   l.ping.pingTimeSeconds(),
		"version":    proto,
		"subver":           sub,
		"inbound":          l.inbound,
		"startingheight":   startH,
		"synced_headers":   peerSyncedHeaders(l, tipH),
		"synced_blocks":    peerSyncedBlocks(l, syncedBlocks),
		"dogego_best_header_hash": strings.TrimSpace(l.bestHeaderHash),
		"dogego_tip_updated":      l.tipUpdatedUnix,
		"connection_type":  connType,
		"restricted":       false,
		"bip152_hb_to":     l.cmpctHBTo,
		"bip152_hb_from":   l.cmpctHBFrom,
		"last_block":       lastBlock,
		"last_transaction": lastTx,
		"dogego_note":      note,
		"inflight":         peerInflightFromSnap(byLane, syncLaneForPeer(l.addr, ctxPrimaryAddr(ctx, l), assist, ctxRaw(ctx))),
	}
	if la := peerLocalAddr(l.conn); la != "" {
		row["addrlocal"] = la
	}
	if minPing := l.ping.minPingSeconds(); minPing > 0 {
		row["minping"] = minPing
	}
	if pingWait := l.ping.pingWaitSeconds(); pingWait > 0 {
		row["pingwait"] = pingWait
	}
	if ctx != nil && ctx.AddedNodes != nil && ctx.AddedNodes.Contains(l.addr) {
		row["addnode"] = true
		row["whitelisted"] = true
	}
	row["addr_processed"] = l.addrProcessed
	row["addr_rate_limited"] = l.addrRateLimited
	if l.msgStats != nil {
		row["bytesrecv_per_msg"] = l.msgStats.recvMap()
		row["bytessent_per_msg"] = l.msgStats.sentMap()
	}
	if ctx != nil && ctx.Misbehavior != nil {
		row["banscore"] = ctx.Misbehavior.Score(l.addr)
	}
	if l.dgrTunneled {
		row["dogego_dgr_tunnel"] = true
	}
	if l.peer != nil && chain.HasDogeGoRelayCGNAT(l.peer.Services) {
		row["dogego_relay_cgnat"] = true
	}
	if l.primary {
		row["dogego_role"] = "primary"
	} else {
		row["dogego_role"] = "relay"
	}
	if ctx != nil && ctx.RawFill != nil {
		lane := syncLaneForPeer(l.addr, ctx.PrimaryAddr, assist, ctx.RawFill)
		if lane >= 0 {
			row["dogego_ibd_lane"] = lane
		}
	}
	if l.peerFeeFilter > 0 {
		row["feefilter"] = rpc.FormatFeeFilterDOGE(l.peerFeeFilter)
	}
	for k, v := range peerInfoExtras(l.addr, ctx) {
		row[k] = v
	}
	if pm != nil && pm.addrs != nil && pm.addrs.IsTried(l.addr) {
		row["dogego_addrbook_tried"] = true
	}
	return row
}

func ctxPrimaryAddr(ctx *PeerInfoContext, l *peerLink) string {
	if ctx != nil && ctx.PrimaryAddr != "" {
		return ctx.PrimaryAddr
	}
	if l != nil && l.primary {
		return l.addr
	}
	if ctx != nil {
		return ctx.PrimaryAddr
	}
	return ""
}

func ctxRaw(ctx *PeerInfoContext) *progressiveRawState {
	if ctx == nil {
		return nil
	}
	return ctx.RawFill
}

func assistPeerInfoRows(ctx *PeerInfoContext, p chain.Params, j *store.HeaderJournal, existing []map[string]interface{}) []map[string]interface{} {
	assist := assistSnapshots(ctx)
	var byLane map[int][]int64
	if ctx != nil && ctx.RawFill != nil {
		byLane = ctx.RawFill.InflightHeightsByLane()
	}
	return assistPeerInfoRowsWithSnap(ctx, p, j, existing, assist, byLane)
}

func assistPeerInfoRowsWithSnap(ctx *PeerInfoContext, p chain.Params, j *store.HeaderJournal, existing []map[string]interface{}, assist []AssistPeerSnapshot, byLane map[int][]int64) []map[string]interface{} {
	if ctx == nil || ctx.Assist == nil || len(assist) == 0 {
		return nil
	}
	var tipH int64
	if j != nil {
		tipH, _ = j.TipHeight()
	}
	syncedBlocks := tipH
	if ctx.SyncedBlocks >= 0 {
		syncedBlocks = ctx.SyncedBlocks
	}
	now := time.Now().Unix()
	seen := make(map[string]struct{}, len(existing))
	for _, row := range existing {
		if a, ok := row["addr"].(string); ok {
			seen[a] = struct{}{}
		}
	}
	var out []map[string]interface{}
	baseID := 9000
	for i, snap := range assist {
		if _, dup := seen[snap.Addr]; dup {
			continue
		}
		row := map[string]interface{}{
			"id":             baseID + i + 1,
			"addr":           snap.Addr,
			"addrlocal":      "",
			"relaytxes":      false,
			"relayaddresses": false,
			"services":       rpc.FormatServicesHex(p.NodeNetwork),
			"lastsend":       now,
			"lastrecv":       now,
			"bytessent":      int64(snap.BytesSent),
			"bytesrecv":      int64(snap.BytesRecv),
			"conntime":         snap.Since.Unix(),
			"timeoffset":       int32(0),
			"pingtime":         0.0,
			"version":          p.ProtocolVersion,
			"subver":           "",
			"inbound":          false,
			"startingheight":   int64(0),
			"synced_headers":   tipH,
			"synced_blocks":    syncedBlocks,
			"connection_type":  "outbound-block-sync",
			"restricted":       false,
			"bip152_hb_to":     false,
			"bip152_hb_from":   false,
			"dogego_note":      "block-assist IBD worker",
			"dogego_role":      "block-assist",
			"dogego_ibd_lane":  snap.Lane,
			"inflight":         peerInflightFromSnap(byLane, snap.Lane),
		}
		for k, v := range peerInfoExtras(snap.Addr, ctx) {
			row[k] = v
		}
		out = append(out, row)
	}
	return out
}

// HasSession reports whether addr has an active P2P session (primary or relay).
func (pm *PeerMgr) HasSession(addr string) bool {
	if pm == nil || addr == "" {
		return false
	}
	pm.mu.Lock()
	_, ok := pm.sessions[addr]
	pm.mu.Unlock()
	return ok
}

// TotalNetBytes returns aggregate recv/sent across all peer links.
func (pm *PeerMgr) TotalNetBytes() (recv, sent int64) {
	if pm == nil {
		return 0, 0
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, l := range pm.sessions {
		if l.ctr == nil {
			continue
		}
		recv += int64(l.ctr.Recv())
		sent += int64(l.ctr.Sent())
	}
	return recv, sent
}
