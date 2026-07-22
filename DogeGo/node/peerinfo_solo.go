// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"time"

	"dogego/rpc"
	"dogego/wire"
)

// SoloPrimaryPeerInfoOpts builds a Core-shaped getpeerinfo row for the single-TCP sync path (no PeerMgr).
type SoloPrimaryPeerInfoOpts struct {
	Addr            string
	Conn            net.Conn
	ConnTime        time.Time
	ProtocolVersion int32
	SubVer          string
	ServicesHex     string
	StartHeight     int64
	TipH            int64
	SyncedBlocks    int64
	Sent            int64
	Recv            int64
	TimeOffset      int32
	Ping            *peerPingTracker
	LastSend        time.Time
	LastRecv        time.Time
	LastBlock       time.Time
	LastTx          time.Time
	FeeFilterKoinu  uint64
	CmpctHBFrom     bool
	CmpctHBTo       bool
	RelayTxes       bool
	MsgWriter       *MsgWriter
	Misbehavior     *MisbehaviorTracker
	Ctx             *PeerInfoContext
	Note            string
	DGRTunneled     bool
}

// BuildSoloPrimaryPeerInfoRow returns one getpeerinfo entry matching PeerInfoMaps field names.
func BuildSoloPrimaryPeerInfoRow(o SoloPrimaryPeerInfoOpts) map[string]interface{} {
	now := time.Now().Unix()
	lastSend := now
	if !o.LastSend.IsZero() {
		lastSend = o.LastSend.Unix()
	}
	lastRecv := now
	if !o.LastRecv.IsZero() {
		lastRecv = o.LastRecv.Unix()
	}
	row := map[string]interface{}{
		"id":               1,
		"addr":             o.Addr,
		"relaytxes":        o.RelayTxes,
		"relayaddresses":   true,
		"services":         o.ServicesHex,
		"lastsend":         lastSend,
		"lastrecv":         lastRecv,
		"bytessent":        o.Sent,
		"bytesrecv":        o.Recv,
		"conntime":         o.ConnTime.Unix(),
		"timeoffset":       o.TimeOffset,
		"pingtime":         o.Ping.pingTimeSeconds(),
		"version":          o.ProtocolVersion,
		"subver":           o.SubVer,
		"inbound":          false,
		"startingheight":   o.StartHeight,
		"synced_headers":   o.TipH,
		"synced_blocks":    o.SyncedBlocks,
		"connection_type":  "outbound-full-relay",
		"restricted":       false,
		"bip152_hb_to":     o.CmpctHBTo,
		"bip152_hb_from":   o.CmpctHBFrom,
		"dogego_note":      o.Note,
		"dogego_role":      "primary",
	}
	if o.DGRTunneled {
		row["dogego_dgr_tunnel"] = true
	}
	if la := peerLocalAddr(o.Conn); la != "" {
		row["addrlocal"] = la
	}
	if minPing := o.Ping.minPingSeconds(); minPing > 0 {
		row["minping"] = minPing
	}
	if pingWait := o.Ping.pingWaitSeconds(); pingWait > 0 {
		row["pingwait"] = pingWait
	}
	if !o.LastBlock.IsZero() {
		row["last_block"] = o.LastBlock.Unix()
	}
	if !o.LastTx.IsZero() {
		row["last_transaction"] = o.LastTx.Unix()
	}
	if o.Ctx != nil && o.Ctx.AddedNodes != nil && o.Ctx.AddedNodes.Contains(o.Addr) {
		row["addnode"] = true
		row["whitelisted"] = true
	}
	if o.Misbehavior != nil {
		row["banscore"] = o.Misbehavior.Score(o.Addr)
	}
	if o.MsgWriter != nil && o.MsgWriter.msgStats != nil {
		row["bytesrecv_per_msg"] = o.MsgWriter.msgStats.recvMap()
		row["bytessent_per_msg"] = o.MsgWriter.msgStats.sentMap()
	}
	row["inflight"] = peerInflightRow(o.Ctx, o.Addr, true)
	if o.FeeFilterKoinu > 0 {
		row["feefilter"] = rpc.FormatFeeFilterDOGE(o.FeeFilterKoinu)
	}
	for k, v := range peerInfoExtras(o.Addr, o.Ctx) {
		row[k] = v
	}
	return row
}

// WireSoloPrimaryBlockPeerStats records getpeerinfo last_block when blocks arrive on the lone sync TCP link.
func WireSoloPrimaryBlockPeerStats(bs *BlockStoreCtx, peerAddr string, onBlock func()) {
	if bs == nil || onBlock == nil {
		return
	}
	bs.OnBlockFromPeer = func(addr string, _ int64) {
		if peerAddr == "" || addr == peerAddr {
			onBlock()
		}
	}
}

func relayTxesFromVersion(dv *wire.DecodedVersion) bool {
	if dv == nil {
		return true
	}
	return dv.RelayTxes
}
