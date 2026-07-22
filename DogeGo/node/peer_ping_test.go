// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"sync"
	"testing"
	"time"

	"dogego/chain"
	"dogego/wire"
)

func TestForcePingRoundTrip(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	mw := NewMsgWriter(c1, p.Magic)
	var ping peerPingTracker
	type pingMsg struct {
		cmd string
		pl  []byte
		err error
	}
	ch := make(chan pingMsg, 1)
	go func() {
		cmd, pl, err := wire.ReadMessage(c2, p.Magic)
		ch <- pingMsg{cmd, pl, err}
	}()
	ping.forcePing(mw)
	msg := <-ch
	cmd, pl, err := msg.cmd, msg.pl, msg.err
	if err != nil || cmd != "ping" || len(pl) != 8 {
		t.Fatalf("read ping: %v cmd=%s", err, cmd)
	}
	time.Sleep(time.Millisecond)
	ping.notePong(pl)
	if ping.pingTimeSeconds() <= 0 {
		t.Fatal("expected pingtime after pong")
	}
}

func TestPingAllSessions(t *testing.T) {
	p := mustTestnetParams(t)
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 8}, p, "/DogeGo/", net.Dialer{})
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	mw1 := NewMsgWriter(c1, p.Magic)
	pm.RegisterPrimary("10.0.0.1:22556", c1, mw1, nil, nil)
	c3, c4 := net.Pipe()
	defer c3.Close()
	defer c4.Close()
	mw2 := NewMsgWriter(c3, p.Magic)
	mw2.PeerAddr = "10.0.0.2:22556"
	pm.mu.Lock()
	pm.sessions["10.0.0.2:22556"] = &peerLink{addr: "10.0.0.2:22556", mw: mw2}
	pm.mu.Unlock()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		expectPing(t, c2, p.Magic)
	}()
	go func() {
		defer wg.Done()
		expectPing(t, c4, p.Magic)
	}()
	pm.PingAll()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for pings")
	}
}

func expectPing(t *testing.T, conn net.Conn, magic [4]byte) {
	t.Helper()
	cmd, pl, err := wire.ReadMessage(conn, magic)
	if err != nil || cmd != "ping" || len(pl) != 8 {
		t.Fatalf("ping read: %v cmd=%s", err, cmd)
	}
}
