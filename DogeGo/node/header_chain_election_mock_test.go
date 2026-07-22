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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func startMockForkProbePeer(t *testing.T, p chain.Params, reply []wire.DecodedHeader) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveMockForkProbePeer(conn, p, reply)
		}
	}()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func serveMockForkProbePeer(conn net.Conn, p chain.Params, reply []wire.DecodedHeader) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := acceptForkProbeHandshake(conn, p); err != nil {
		return
	}
	for {
		cmd, pl, err := wire.ReadMessage(conn, p.Magic)
		if err != nil {
			return
		}
		switch cmd {
		case "ping":
			_ = wire.WriteMessage(conn, p.Magic, "pong", pl)
		case "sendheaders", "sendcmpct", "verack":
		case "getheaders":
			body, err := wire.EncodeHeadersPayload(reply)
			if err != nil {
				return
			}
			_ = wire.WriteMessage(conn, p.Magic, "headers", body)
			return
		}
	}
}

func acceptForkProbeHandshake(conn net.Conn, p chain.Params) error {
	gotClientVer := false
	for {
		cmd, pl, err := wire.ReadMessage(conn, p.Magic)
		if err != nil {
			return err
		}
		switch cmd {
		case "version":
			if !gotClientVer {
				gotClientVer = true
				sv := wire.BuildVersionPayload(p.ProtocolVersion, p.NodeNetwork, nil, 0, 99, "/mock/", 100, true)
				if err := wire.WriteMessage(conn, p.Magic, "version", sv); err != nil {
					return err
				}
				if err := wire.WriteMessage(conn, p.Magic, "verack", nil); err != nil {
					return err
				}
			}
		case "verack":
			return nil
		case "reject":
			rj, err := wire.DecodeRejectPayload(pl)
			if err != nil {
				return err
			}
			return fmt.Errorf("reject: %s", rj.String())
		case "sendheaders", "sendcmpct":
		}
	}
}

func testHeaderJournalWithTip(t *testing.T) (*store.HeaderJournal, [32]byte, chain.Params) {
	t.Helper()
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x01
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	return j, genHash, p
}

func makeExtendingHeader(forkPrev [32]byte, nonce byte, bits uint32) wire.DecodedHeader {
	h := make([]byte, 80)
	copy(h[4:36], forkPrev[:])
	h[76] = nonce
	setHeaderBits(h, bits)
	return wire.DecodedHeader{Header80: h}
}

func registerRelayPeer(pm *PeerMgr, p chain.Params, addr string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	fc := &fakeConn{}
	pm.sessions[addr] = &peerLink{addr: addr, mw: NewMsgWriter(fc, p.Magic)}
	pm.order = append(pm.order, addr)
}

func TestMockForkProbePeerRoundTrip(t *testing.T) {
	j, genHash, p := testHeaderJournalWithTip(t)
	peerReply := []wire.DecodedHeader{makeExtendingHeader(genHash, 0x22, testHardBits)}
	addr, stop := startMockForkProbePeer(t, p, peerReply)
	defer stop()
	payload, err := encodeForkProbeGetHeaders(j, 0, p)
	if err != nil {
		t.Fatal(err)
	}
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, p, "test", net.Dialer{})
	decoded, err := pm.syncForkProbeHeaders(context.Background(), addr, p, payload, genHash)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("decoded len %d", len(decoded))
	}
}

func TestEnsureIncomingForkWins_rejectsPeerAlternate(t *testing.T) {
	j, genHash, p := testHeaderJournalWithTip(t)
	incoming := []wire.DecodedHeader{makeExtendingHeader(genHash, 0x11, testEasyBits)}
	incomingWork, err := incomingChainWork(incoming)
	if err != nil {
		t.Fatal(err)
	}
	peerReply := []wire.DecodedHeader{makeExtendingHeader(genHash, 0x22, testHardBits)}
	addr, stop := startMockForkProbePeer(t, p, peerReply)
	defer stop()

	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, p, "test", net.Dialer{})
	registerRelayPeer(pm, p, addr)

	err = pm.EnsureIncomingForkWins(context.Background(), j, p, 0, genHash, incoming, incomingWork)
	if err == nil {
		t.Fatal("expected fork reject when peer alternate has more work")
	}
	if !strings.Contains(err.Error(), "fork rejected") {
		t.Fatalf("err %v", err)
	}
}

func TestEnsureIncomingForkWins_acceptsIncomingWinner(t *testing.T) {
	j, genHash, p := testHeaderJournalWithTip(t)
	alt := makeExtendingHeader(genHash, 0x22, testHardBits)
	altHash := pow.BlockHashLE(alt.Header80)
	alt2 := make([]byte, 80)
	copy(alt2, alt.Header80)
	copy(alt2[4:36], altHash[:])
	alt2[76] ^= 0x33
	setHeaderBits(alt2, testHardBits)
	incoming := []wire.DecodedHeader{alt, {Header80: alt2}}
	incomingWork, err := incomingChainWork(incoming)
	if err != nil {
		t.Fatal(err)
	}
	peerReply := []wire.DecodedHeader{makeExtendingHeader(genHash, 0x11, testEasyBits)}
	addr, stop := startMockForkProbePeer(t, p, peerReply)
	defer stop()

	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, p, "test", net.Dialer{})
	registerRelayPeer(pm, p, addr)

	if err := pm.EnsureIncomingForkWins(context.Background(), j, p, 0, genHash, incoming, incomingWork); err != nil {
		t.Fatalf("incoming should win: %v", err)
	}
}
