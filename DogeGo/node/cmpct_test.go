// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
	"dogego/primitives"
	"dogego/rpc"
	"dogego/store"
	"dogego/wire"
)

func TestNegotiateSendCmpct_enablesHBTo(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, p, "/DogeGo/", net.Dialer{})
	cSrv, cCli := net.Pipe()
	defer cSrv.Close()
	defer cCli.Close()
	mw := NewMsgWriter(cSrv, p.Magic)
	link := &peerLink{addr: "10.0.0.1:22556", mw: mw}
	pm.mu.Lock()
	pm.sessions[link.addr] = link
	pm.mu.Unlock()

	peerBody, err := wire.EncodeSendCmpct(true, 1)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan []byte, 1)
	go func() {
		_ = cCli.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			t.Error(err)
			close(done)
			return
		}
		done <- pl
	}()

	peerAnn, weAnn, err := NegotiateSendCmpct(pm, link, mw, peerBody, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !peerAnn || !weAnn || !link.cmpctHBTo || !link.cmpctHBFrom {
		t.Fatalf("peerAnn=%v weAnn=%v hbTo=%v hbFrom=%v", peerAnn, weAnn, link.cmpctHBTo, link.cmpctHBFrom)
	}
	pl := <-done
	sc, err := wire.DecodeSendCmpct(pl)
	if err != nil || !sc.Announce || sc.Version != 1 {
		t.Fatalf("reply %+v err %v", sc, err)
	}
}

func TestHandleInboundGetData_CmpctBlock(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	blockRaw, hash := store.TestMinimalBlock()
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			t.Error(err)
			return
		}
		if cmd != "cmpctblock" {
			t.Errorf("cmd %q want cmpctblock", cmd)
			return
		}
		hs, err := wire.DecodeHeaderAndShortIDs(pl)
		if err != nil {
			t.Error(err)
			return
		}
		if !bytes.Equal(hs.Header80[:], blockRaw[:80]) {
			t.Error("header mismatch")
		}
		if len(hs.Prefilled) != 1 || hs.Prefilled[0].Index != 0 {
			t.Errorf("prefilled %+v", hs.Prefilled)
		}
	}()

	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeCmpctBlock, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetBlockTxn_servesMissingTx(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	blockRaw, hash := store.TestMinimalBlock()
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			t.Error(err)
			return
		}
		if cmd != "blocktxn" {
			t.Errorf("cmd %q want blocktxn", cmd)
			return
		}
		bt, err := wire.DecodeBlockTransactions(pl)
		if err != nil || bt.BlockHash != hash || len(bt.Transactions) != 1 {
			t.Errorf("blocktxn %+v err %v", bt, err)
		}
	}()

	req, err := wire.EncodeBlockTransactionsRequest(&wire.BlockTransactionsRequest{
		BlockHash: hash,
		Indexes:   []uint64{0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetBlockTxn(mw, raw, req); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestBuildCmpctBlockPayload_roundtripWithMempool(t *testing.T) {
	blockRaw, _ := store.TestMinimalBlock()
	pl, err := BuildCmpctBlockPayload(blockRaw)
	if err != nil {
		t.Fatal(err)
	}
	hs, err := wire.DecodeHeaderAndShortIDs(pl)
	if err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(10)
	shortHit := wire.MatchCmpctShortIDsFromMempool(hs, pool.RawBlobs())
	raw, err := wire.ReconstructBlockFromCmpct(hs, shortHit, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, blockRaw) {
		t.Fatal("roundtrip block mismatch")
	}
}

func TestAnnounceBlockHash_cmpctToHBPeerOnly(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, p, "/DogeGo/", net.Dialer{})

	hbSrv, hbCli := net.Pipe()
	invSrv, invCli := net.Pipe()
	defer hbSrv.Close()
	defer invSrv.Close()
	hbWriter := NewMsgWriter(hbSrv, p.Magic)
	invWriter := NewMsgWriter(invSrv, p.Magic)
	pm.mu.Lock()
	pm.sessions["hb:1"] = &peerLink{addr: "hb:1", mw: hbWriter, cmpctHBTo: true}
	pm.sessions["inv:1"] = &peerLink{addr: "inv:1", mw: invWriter, cmpctHBTo: false}
	pm.mu.Unlock()

	blockRaw, hash := store.TestMinimalBlock()
	hbDone := make(chan string, 1)
	invDone := make(chan string, 1)
	go readOneCmd(t, hbCli, p, hbDone)
	go readOneCmd(t, invCli, p, invDone)

	AnnounceBlockHash(BlockAnnounceEnv{PeerMgr: pm}, hash, blockRaw, "")

	if cmd := <-hbDone; cmd != "cmpctblock" {
		t.Fatalf("hb peer cmd %q", cmd)
	}
	if cmd := <-invDone; cmd != "inv" {
		t.Fatalf("inv peer cmd %q", cmd)
	}
}

func TestAnnounceBlockHash_excludesSourcePeer(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, p, "/DogeGo/", net.Dialer{})

	srcSrv, srcCli := net.Pipe()
	hbSrv, hbCli := net.Pipe()
	defer srcSrv.Close()
	defer hbSrv.Close()
	srcMW := NewMsgWriter(srcSrv, p.Magic)
	hbMW := NewMsgWriter(hbSrv, p.Magic)
	pm.mu.Lock()
	pm.sessions["src:1"] = &peerLink{addr: "src:1", mw: srcMW, cmpctHBTo: false}
	pm.sessions["hb:1"] = &peerLink{addr: "hb:1", mw: hbMW, cmpctHBTo: true}
	pm.mu.Unlock()

	blockRaw, hash := store.TestMinimalBlock()
	srcDone := make(chan struct{}, 1)
	hbDone := make(chan string, 1)
	go func() {
		_ = srcCli.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, _, err := wire.ReadMessage(srcCli, p.Magic); err == nil {
			t.Error("source peer should not receive relay announce")
		}
		srcDone <- struct{}{}
	}()
	go readOneCmd(t, hbCli, p, hbDone)

	AnnounceBlockHash(BlockAnnounceEnv{PeerMgr: pm}, hash, blockRaw, "src:1")
	<-srcDone
	if cmd := <-hbDone; cmd != "cmpctblock" {
		t.Fatalf("hb peer cmd %q", cmd)
	}
}

func TestRelayStoredBlock_announcesHBPeer(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, p, "/DogeGo/", net.Dialer{})
	dir := t.TempDir()
	rawStore, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	blockRaw, hash := store.TestMinimalBlock()
	if err := rawStore.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(nil, nil, p, rawStore, nil, nil)
	bs.announce = BlockAnnounceEnv{PeerMgr: pm}

	hbSrv, hbCli := net.Pipe()
	defer hbSrv.Close()
	defer hbCli.Close()
	hbMW := NewMsgWriter(hbSrv, p.Magic)
	pm.mu.Lock()
	pm.sessions["hb:1"] = &peerLink{addr: "hb:1", mw: hbMW, cmpctHBTo: true}
	pm.mu.Unlock()
	hbDone := make(chan string, 1)
	go readOneCmd(t, hbCli, p, hbDone)

	RelayStoredBlock(bs, blockRaw, "src:1")
	if cmd := <-hbDone; cmd != "cmpctblock" {
		t.Fatalf("hb peer cmd %q want cmpctblock", cmd)
	}
}

func TestCmpctHBSessionCounts(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, p, "/DogeGo/", net.Dialer{})
	pm.mu.Lock()
	pm.sessions["a"] = &peerLink{addr: "a", cmpctHBTo: true, cmpctHBFrom: true}
	pm.sessions["b"] = &peerLink{addr: "b", cmpctHBTo: true}
	pm.sessions["c"] = &peerLink{addr: "c", cmpctHBFrom: true}
	pm.mu.Unlock()
	to, from := pm.CmpctHBSessionCounts()
	if to != 2 || from != 2 {
		t.Fatalf("session counts to=%d from=%d want 2/2", to, from)
	}
	out := map[string]any{}
	annotateCmpctHBCounts(out, pm, true, false)
	if out["bip152_hb_to"] != 3 || out["bip152_hb_from"] != 2 {
		t.Fatalf("annotated counts: %#v", out)
	}
	if out["bip152_hb_max"] != maxCmpctHBPeers {
		t.Fatalf("max: %#v", out["bip152_hb_max"])
	}
}

func TestAnnounceBlockHash_auxpowUsesInvOnly(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, p, "/DogeGo/", net.Dialer{})
	hbSrv, hbCli := net.Pipe()
	defer hbSrv.Close()
	defer hbCli.Close()
	hbMW := NewMsgWriter(hbSrv, p.Magic)
	pm.mu.Lock()
	pm.sessions["hb:1"] = &peerLink{addr: "hb:1", mw: hbMW, cmpctHBTo: true}
	pm.mu.Unlock()

	blockRaw := minimalAuxpowBlockNodeTest(t)
	hash := pow.BlockHashLE(blockRaw[:80])
	hbDone := make(chan string, 1)
	go readOneCmd(t, hbCli, p, hbDone)

	AnnounceBlockHash(BlockAnnounceEnv{PeerMgr: pm}, hash, blockRaw, "")
	if cmd := <-hbDone; cmd != "inv" {
		t.Fatalf("auxpow block cmd %q want inv", cmd)
	}
}

func minimalAuxpowBlockNodeTest(t *testing.T) []byte {
	t.Helper()
	inner := minimalAuxPowBytesWireTest(t)
	hdr := make([]byte, 80)
	binary.LittleEndian.PutUint32(hdr[0:4], 1|(1<<8))
	var coinbase bytes.Buffer
	_ = wire.WriteCompactSize(&coinbase, 1)
	var z [32]byte
	_, _ = coinbase.Write(z[:])
	_ = binary.Write(&coinbase, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&coinbase, 1)
	_, _ = coinbase.Write([]byte{0x00})
	_ = wire.WriteCompactSize(&coinbase, 1)
	_ = binary.Write(&coinbase, binary.LittleEndian, int64(1))
	_ = wire.WriteCompactSize(&coinbase, 1)
	_, _ = coinbase.Write([]byte{0x51})
	_ = binary.Write(&coinbase, binary.LittleEndian, uint32(0))
	var block bytes.Buffer
	_, _ = block.Write(hdr)
	_, _ = block.Write(inner)
	_ = wire.WriteCompactSize(&block, 1)
	_, _ = block.Write(coinbase.Bytes())
	return block.Bytes()
}

func minimalAuxPowBytesWireTest(t *testing.T) []byte {
	t.Helper()
	var coinbase []byte
	{
		var cb bytes.Buffer
		_ = wire.WriteCompactSize(&cb, 1)
		var z [32]byte
		_, _ = cb.Write(z[:])
		_ = binary.Write(&cb, binary.LittleEndian, uint32(0xffffffff))
		_ = wire.WriteCompactSize(&cb, 1)
		_, _ = cb.Write([]byte{0x00})
		_ = wire.WriteCompactSize(&cb, 1)
		_ = binary.Write(&cb, binary.LittleEndian, int64(1))
		_ = wire.WriteCompactSize(&cb, 1)
		_, _ = cb.Write([]byte{0x51})
		_ = binary.Write(&cb, binary.LittleEndian, uint32(0))
		coinbase = cb.Bytes()
	}
	var b bytes.Buffer
	_, _ = b.Write(coinbase)
	var z [32]byte
	_, _ = b.Write(z[:])
	_ = wire.WriteCompactSize(&b, 0)
	_ = binary.Write(&b, binary.LittleEndian, int32(-1))
	_ = wire.WriteCompactSize(&b, 0)
	_ = binary.Write(&b, binary.LittleEndian, int32(0))
	var parent [80]byte
	_, _ = b.Write(parent[:])
	return b.Bytes()
}

func readOneCmd(t *testing.T, c net.Conn, p chain.Params, out chan<- string) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	cmd, _, err := wire.ReadMessage(c, p.Magic)
	if err != nil {
		t.Error(err)
		out <- ""
		return
	}
	out <- cmd
}

func TestCmpctInboundFromPeer_mempoolReconstruct(t *testing.T) {
	blockRaw, tx2raw := testTwoTxBlock(t)
	cmpctPL, err := BuildCmpctBlockPayload(blockRaw)
	if err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(10)
	if err := pool.Add(tx2raw); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, p, "/DogeGo/", net.Dialer{})
	cSrv, cCli := net.Pipe()
	defer cSrv.Close()
	defer cCli.Close()
	mw := NewMsgWriter(cSrv, p.Magic)
	link := &peerLink{addr: "peer:1", cmpctHBFrom: true, mw: mw}

	dir := t.TempDir()
	rawStore, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(nil, nil, p, rawStore, nil, nil)
	env := CmpctServeEnv{Raw: rawStore, Pool: pool, Block: bs}
	hbSrv, hbCli := net.Pipe()
	defer hbSrv.Close()
	defer hbCli.Close()
	hbMW := NewMsgWriter(hbSrv, p.Magic)
	bs.announce = BlockAnnounceEnv{PeerMgr: pm}
	pm.mu.Lock()
	pm.sessions["hb:1"] = &peerLink{addr: "hb:1", mw: hbMW, cmpctHBTo: true}
	pm.mu.Unlock()
	hbDone := make(chan string, 1)
	go readOneCmd(t, hbCli, p, hbDone)

	HandleInboundCmpctBlock(mw, env, link, cmpctPL)

	wantID := testBlockHashLE(blockRaw)
	got, err := rawStore.Get(wantID)
	if err != nil {
		t.Fatalf("block not stored: %v", err)
	}
	if !bytes.Equal(got, blockRaw) {
		t.Fatal("stored block mismatch")
	}
	if cmd := <-hbDone; cmd != "cmpctblock" {
		t.Fatalf("relay hb peer cmd %q want cmpctblock", cmd)
	}
}

func TestCmpctInboundFromPeer_getblocktxnRoundtrip(t *testing.T) {
	blockRaw, _ := testTwoTxBlock(t)
	cmpctPL, err := BuildCmpctBlockPayload(blockRaw)
	if err != nil {
		t.Fatal(err)
	}
	hs, err := wire.DecodeHeaderAndShortIDs(cmpctPL)
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	rawStore, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rawStore.Put(testBlockHashLE(blockRaw), blockRaw); err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(nil, nil, p, rawStore, nil, nil)
	pool := mempool.New(10)
	link := &peerLink{addr: "peer:1", cmpctHBFrom: true}

	blockID := testBlockHashLE(blockRaw)
	link.cmpctPending = &cmpctPending{
		header:   hs,
		blockID:  blockID,
		missing:  []uint64{1},
		shortHit: map[uint64][]byte{},
	}
	txs, err := BlockTxRawsAtIndexes(blockRaw, []uint64{1})
	if err != nil {
		t.Fatal(err)
	}
	btPL, err := wire.EncodeBlockTransactions(&wire.BlockTransactions{
		BlockHash: blockID, Transactions: txs,
	})
	if err != nil {
		t.Fatal(err)
	}
	env := CmpctServeEnv{Raw: rawStore, Pool: pool, Block: bs}
	HandleInboundBlockTxn(nil, env, link, btPL)

	got, err := rawStore.Get(blockID)
	if err != nil {
		t.Fatalf("block not stored: %v", err)
	}
	if !bytes.Equal(got, blockRaw) {
		t.Fatal("stored block mismatch after getblocktxn roundtrip")
	}
	if link.cmpctPending != nil {
		t.Fatal("pending cmpct state not cleared")
	}
}

func TestNegotiateSendCmpct_fourthPeerDeclinesHB(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, p, "/DogeGo/", net.Dialer{})
	peerBody, err := wire.EncodeSendCmpct(true, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxCmpctHBPeers; i++ {
		cSrv, cCli := net.Pipe()
		mw := NewMsgWriter(cSrv, p.Magic)
		addr := "hb-fill:" + strconv.Itoa(i)
		link := &peerLink{addr: addr, mw: mw}
		pm.mu.Lock()
		pm.sessions[link.addr] = link
		pm.mu.Unlock()
		go func(c net.Conn) {
			_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, _, _ = wire.ReadMessage(c, p.Magic)
			_ = c.Close()
		}(cCli)
		_, weAnn, err := NegotiateSendCmpct(pm, link, mw, peerBody, nil)
		_ = cSrv.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !weAnn || !link.cmpctHBTo {
			t.Fatalf("slot %d: weAnn=%v hbTo=%v", i, weAnn, link.cmpctHBTo)
		}
	}
	cSrv, cCli := net.Pipe()
	defer cSrv.Close()
	defer cCli.Close()
	mw := NewMsgWriter(cSrv, p.Magic)
	link := &peerLink{addr: "hb-overflow:1", mw: mw}
	pm.mu.Lock()
	pm.sessions[link.addr] = link
	pm.mu.Unlock()
	done := make(chan wire.SendCmpct, 1)
	go func() {
		_ = cCli.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			t.Error(err)
			done <- wire.SendCmpct{}
			return
		}
		sc, err := wire.DecodeSendCmpct(pl)
		if err != nil {
			t.Error(err)
			done <- wire.SendCmpct{}
			return
		}
		done <- sc
	}()
	_, weAnn, err := NegotiateSendCmpct(pm, link, mw, peerBody, nil)
	if err != nil {
		t.Fatal(err)
	}
	if weAnn || link.cmpctHBTo {
		t.Fatalf("4th peer should decline HB: weAnn=%v hbTo=%v", weAnn, link.cmpctHBTo)
	}
	sc := <-done
	if sc.Announce {
		t.Fatalf("reply announce=%v want false", sc)
	}
}

func TestNoteCmpctWireIgnored_firstSilentThenMisbehavior(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	mb := NewMisbehaviorTracker(rpc.NewMemoryBanManager())
	cSrv, cCli := net.Pipe()
	defer cSrv.Close()
	defer cCli.Close()
	mw := NewMsgWriter(cSrv, p.Magic)
	link := &peerLink{addr: "10.0.0.9:22556", mw: mw}

	link.NoteCmpctWireIgnored(mw, "cmpctblock", mb)
	if mb.Score(link.addr) != 0 {
		t.Fatalf("first ignore should not score, got %d", mb.Score(link.addr))
	}
	if !link.cmpctWireIgnored {
		t.Fatal("expected cmpctWireIgnored after first notice")
	}

	done := make(chan string, 1)
	go func() {
		_ = cCli.SetReadDeadline(time.Now().Add(3 * time.Second))
		cmd, _, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			done <- "err:" + err.Error()
			return
		}
		done <- cmd
	}()
	link.NoteCmpctWireIgnored(mw, "getblocktxn", mb)
	if mb.Score(link.addr) != misbehaviorUnexpectedCompact {
		t.Fatalf("score=%d want %d", mb.Score(link.addr), misbehaviorUnexpectedCompact)
	}
	if cmd := <-done; cmd != "reject" {
		t.Fatalf("cmd %q want reject", cmd)
	}
}

func TestHandleInboundCmpctBlock_nonHBIgnored(t *testing.T) {
	blockRaw, _ := testTwoTxBlock(t)
	cmpctPL, err := BuildCmpctBlockPayload(blockRaw)
	if err != nil {
		t.Fatal(err)
	}
	before := cmpctMetrics.In.Load()
	link := &peerLink{addr: "nohb:1", cmpctHBFrom: false}
	HandleInboundCmpctBlock(nil, CmpctServeEnv{}, link, cmpctPL)
	if cmpctMetrics.In.Load() != before {
		t.Fatal("non-HB cmpctblock should not increment dogego_cmpct_in")
	}
}

func TestHandleInboundCmpctBlock_auxpowFallsBackToFullBlock(t *testing.T) {
	// Craft a cmpctblock whose header has the AuxPoW version bit; reconstruct must fail → full getdata.
	blockRaw, _ := testTwoTxBlock(t)
	cmpctPL, err := BuildCmpctBlockPayload(blockRaw)
	if err != nil {
		t.Fatal(err)
	}
	hs, err := wire.DecodeHeaderAndShortIDs(cmpctPL)
	if err != nil {
		t.Fatal(err)
	}
	ver := binary.LittleEndian.Uint32(hs.Header80[0:4])
	binary.LittleEndian.PutUint32(hs.Header80[0:4], ver|(1<<8))
	// Prefill-only (drop short IDs) so reconstruct is attempted immediately without getblocktxn.
	hs.ShortIDs = nil
	hs.Prefilled = hs.Prefilled[:1]
	cmpctPL, err = wire.EncodeHeaderAndShortIDs(hs)
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	defer cSrv.Close()
	defer cCli.Close()
	mw := NewMsgWriter(cSrv, p.Magic)
	link := &peerLink{addr: "peer:aux", cmpctHBFrom: true, mw: mw}
	before := cmpctMetrics.ReconstructFallback.Load()
	done := make(chan string, 1)
	go func() {
		_ = cCli.SetReadDeadline(time.Now().Add(3 * time.Second))
		cmd, _, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			done <- "err:" + err.Error()
			return
		}
		done <- cmd
	}()
	HandleInboundCmpctBlock(mw, CmpctServeEnv{}, link, cmpctPL)
	if cmpctMetrics.ReconstructFallback.Load() != before+1 {
		t.Fatalf("fallback metric %d want %d", cmpctMetrics.ReconstructFallback.Load(), before+1)
	}
	if cmd := <-done; cmd != "getdata" {
		t.Fatalf("cmd %q want getdata", cmd)
	}
}

func TestCmpctInboundBlockTxnMismatchFallsBackToFullBlock(t *testing.T) {
	blockRaw, _ := testTwoTxBlock(t)
	cmpctPL, err := BuildCmpctBlockPayload(blockRaw)
	if err != nil {
		t.Fatal(err)
	}
	hs, err := wire.DecodeHeaderAndShortIDs(cmpctPL)
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	defer cSrv.Close()
	defer cCli.Close()
	mw := NewMsgWriter(cSrv, p.Magic)
	blockID := testBlockHashLE(blockRaw)
	link := &peerLink{addr: "peer:fb", cmpctHBFrom: true, cmpctPending: &cmpctPending{
		header:   hs,
		blockID:  blockID,
		missing:  []uint64{1},
		shortHit: map[uint64][]byte{},
	}}
	before := cmpctMetrics.ReconstructFallback.Load()
	done := make(chan string, 1)
	go func() {
		_ = cCli.SetReadDeadline(time.Now().Add(3 * time.Second))
		cmd, _, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			done <- "err:" + err.Error()
			return
		}
		done <- cmd
	}()
	// Wrong tx count → fallback getdata MSG_BLOCK
	btPL, err := wire.EncodeBlockTransactions(&wire.BlockTransactions{
		BlockHash: blockID, Transactions: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	HandleInboundBlockTxn(mw, CmpctServeEnv{}, link, btPL)
	if link.cmpctPending != nil {
		t.Fatal("pending not cleared")
	}
	if cmpctMetrics.ReconstructFallback.Load() != before+1 {
		t.Fatalf("fallback metric %d want %d", cmpctMetrics.ReconstructFallback.Load(), before+1)
	}
	if cmd := <-done; cmd != "getdata" {
		t.Fatalf("cmd %q want getdata", cmd)
	}
}

func testTwoTxBlock(t *testing.T) (blockRaw, tx2raw []byte) {
	t.Helper()
	var coin bytes.Buffer
	_ = binary.Write(&coin, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(&coin, 1)
	var zeros [32]byte
	_, _ = coin.Write(zeros[:])
	_ = binary.Write(&coin, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&coin, 1)
	_, _ = coin.Write([]byte{0x00})
	_ = binary.Write(&coin, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&coin, 1)
	_ = binary.Write(&coin, binary.LittleEndian, int64(8800000000))
	_ = wire.WriteCompactSize(&coin, 2)
	_, _ = coin.Write([]byte{0x51, 0x51})
	_ = binary.Write(&coin, binary.LittleEndian, uint32(0))
	coinRaw := coin.Bytes()
	rt := bytes.NewReader(coinRaw)
	tx1, err := wire.ReadTx(rt)
	if err != nil {
		t.Fatal(err)
	}
	tx2 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: tx1.TxHash(), PrevIdx: 0, Script: []byte{0x51}, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51, 0x51}}},
	}
	tx2raw, err = tx2.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	merkle := wire.HashPair(tx1.TxHash(), tx2.TxHash())
	hdr := primitives.BlockHeader{
		Version: 1, PrevBlock: [32]byte{}, MerkleRoot: merkle,
		Timestamp: 1747000000, Bits: 0x1e0ffff0, Nonce: 42,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, 2)
	_, _ = block.Write(coinRaw)
	_, _ = block.Write(tx2raw)
	return block.Bytes(), tx2raw
}

func testBlockHashLE(blockRaw []byte) [32]byte {
	return pow.BlockHashLE(blockRaw[:80])
}
