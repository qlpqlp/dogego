// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"
	"time"

	"dogego/chain"
	"dogego/rpc"
	"dogego/wire"
)

func TestPeerInfoSyncedBlocksFromContext(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &PeerInfoContext{SyncedBlocks: 42, IBDLanes: 7, Scorer: NewBlockPeerScorer(), Assist: NewAssistPeerRegistry()}
	ctx.Scorer.NoteBlocksDelivered("assist:1", 1)
	if ctr := ctx.Assist.Register("assist:1", 3); ctr != nil {
		ctr.addRecv(200)
		ctr.addSent(80)
	}
	rows := assistPeerInfoRows(ctx, p, nil, nil)
	if len(rows) != 1 {
		t.Fatalf("rows %d", len(rows))
	}
	if rows[0]["synced_blocks"].(int64) != 42 {
		t.Fatalf("synced_blocks %#v", rows[0]["synced_blocks"])
	}
	if rows[0]["dogego_ibd_lane"].(int) != 3 {
		t.Fatalf("lane %#v", rows[0]["dogego_ibd_lane"])
	}
	if rows[0]["bytesrecv"].(int64) != 200 || rows[0]["bytessent"].(int64) != 80 {
		t.Fatalf("assist bytes recv=%v sent=%v", rows[0]["bytesrecv"], rows[0]["bytessent"])
	}
	if rows[0]["dogego_block_score"].(int) <= 0 {
		t.Fatalf("score %#v", rows[0]["dogego_block_score"])
	}
}

func TestPeerInfoTimeOffset(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{Mode: P2PModeBoth, MaxOutbound: 4, Listen: true}, p, "/dogego/", net.Dialer{})
	now := time.Now().Unix()
	dv := &wire.DecodedVersion{Timestamp: now + 120}
	pm.RegisterPrimary("93.184.216.1:22556", nil, nil, nil, dv)
	rows := pm.PeerInfoMaps(nil, p, nil)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	off := rows[0]["timeoffset"].(int32)
	if off < 100 || off > 140 {
		t.Fatalf("timeoffset %d", off)
	}
}

func TestPeerInfoLastBlockAndTx(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{Mode: P2PModeBoth, MaxOutbound: 4, Listen: true}, p, "/dogego/", net.Dialer{})
	pm.RegisterPrimary("93.184.216.1:22556", nil, nil, nil, nil)
	pm.NotePeerBlock("93.184.216.1:22556")
	pm.NotePeerTx("93.184.216.1:22556")
	rows := pm.PeerInfoMaps(nil, p, nil)
	if len(rows) != 1 {
		t.Fatalf("rows %d", len(rows))
	}
	if rows[0]["last_block"].(int64) == 0 {
		t.Fatal("last_block should be set")
	}
	if rows[0]["last_transaction"].(int64) == 0 {
		t.Fatal("last_transaction should be set")
	}
}

func TestPeerInfoRelayTxesAndAddnode(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{Mode: P2PModeBoth, MaxOutbound: 4, Listen: true}, p, "/dogego/", net.Dialer{})
	added := NewAddedNodeStore()
	added.Add("93.184.216.1:22556")
	dv := &wire.DecodedVersion{RelayTxes: false}
	pm.RegisterPrimary("93.184.216.1:22556", nil, nil, nil, dv)
	ctx := &PeerInfoContext{AddedNodes: added}
	rows := pm.PeerInfoMaps(nil, p, ctx)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	if rows[0]["relaytxes"].(bool) {
		t.Fatal("relaytxes")
	}
	if !rows[0]["addnode"].(bool) {
		t.Fatal("addnode")
	}
	if !rows[0]["whitelisted"].(bool) {
		t.Fatal("whitelisted")
	}
}

func TestPeerInfoPerMsgAndAddrGossip(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{Mode: P2PModeBoth, MaxOutbound: 4, Listen: true}, p, "/dogego/", net.Dialer{})
	pm.RegisterPrimary("93.184.216.1:22556", nil, nil, nil, nil)
	pm.NotePeerMsg("93.184.216.1:22556", "version", 80)
	good := wire.NetAddress{IP: net.ParseIP("93.184.216.34"), Port: 22556, Time: 1, Services: 1}
	bad := wire.NetAddress{IP: net.ParseIP("127.0.0.1"), Port: 22556, Time: 1, Services: 1}
	pm.NoteAddrsFromPeer("93.184.216.1:22556", []wire.NetAddress{good, bad})
	rows := pm.PeerInfoMaps(nil, p, nil)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	recv, ok := rows[0]["bytesrecv_per_msg"].(map[string]int64)
	if !ok || recv["version"] != p2pFrameBytes(80) {
		t.Fatalf("bytesrecv_per_msg %#v", rows[0]["bytesrecv_per_msg"])
	}
	if rows[0]["addr_processed"].(uint64) != 2 {
		t.Fatalf("addr_processed %#v", rows[0]["addr_processed"])
	}
	if rows[0]["addr_rate_limited"].(uint64) != 0 {
		t.Fatalf("addr_rate_limited %#v", rows[0]["addr_rate_limited"])
	}
}

func TestPeerInfoLastSendAndBanscore(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{Mode: P2PModeBoth, MaxOutbound: 4, Listen: true}, p, "/dogego/", net.Dialer{})
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	readDone := make(chan error, 1)
	go func() {
		_, _, err := wire.ReadMessage(c2, p.Magic)
		readDone <- err
	}()
	mw := NewMsgWriter(c1, p.Magic)
	pm.RegisterPrimary("93.184.216.1:22556", c1, mw, nil, nil)
	if err := mw.Write("ping", []byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
	mb := NewMisbehaviorTracker(rpc.NewMemoryBanManager())
	mb.Note("93.184.216.1:22556", misbehaviorInvalidHeaders, "test")
	ctx := &PeerInfoContext{Misbehavior: mb}
	rows := pm.PeerInfoMaps(nil, p, ctx)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	if rows[0]["lastsend"].(int64) == 0 {
		t.Fatal("lastsend")
	}
	if rows[0]["banscore"].(int) <= 0 {
		t.Fatalf("banscore %#v", rows[0]["banscore"])
	}
}

func TestLocalAddressRowsListenAndSession(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{Mode: P2PModeClassic, MaxOutbound: 4, Listen: true, MaxInbound: 4}, p, "/dogego/", net.Dialer{})
	pm.mu.Lock()
	pm.listenHost = "0.0.0.0"
	pm.listenPort = int(p.Port)
	pm.mu.Unlock()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	pm.RegisterPrimary("93.184.216.1:22556", c1, NewMsgWriter(c1, p.Magic), nil, nil)
	rows := pm.LocalAddressRows()
	if len(rows) < 1 {
		t.Fatalf("rows %#v", rows)
	}
	foundListen := false
	for _, r := range rows {
		if r["port"] == int(p.Port) {
			foundListen = true
		}
	}
	if !foundListen {
		t.Fatalf("listen port missing: %#v", rows)
	}
}

func TestMsgWriterCountsSentPerCmd(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	readDone := make(chan error, 1)
	go func() {
		_, _, err := wire.ReadMessage(c2, p.Magic)
		readDone <- err
	}()
	w := NewMsgWriter(c1, p.Magic)
	attachPeerMsgStats(nil, w)
	if err := w.Write("ping", []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
	sent := w.msgStats.sentMap()
	if sent["ping"] != p2pFrameBytes(3) {
		t.Fatalf("sent map %#v", sent)
	}
}
