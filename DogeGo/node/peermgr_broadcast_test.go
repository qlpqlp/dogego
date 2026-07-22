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
	"dogego/mempool"
	"dogego/wire"
)

func TestBroadcastTxExcludesSource(t *testing.T) {
	magic := [4]byte{0xc0, 0xc0, 0xc0, 0xc0}
	srcC, srcS := net.Pipe()
	relayC, relayS := net.Pipe()
	defer srcC.Close()
	defer srcS.Close()
	defer relayC.Close()
	defer relayS.Close()

	pm := NewPeerMgr(P2PModeSettings{}, chain.Params{}, "test", net.Dialer{})
	const srcAddr = "93.184.216.1:22556"
	const relayAddr = "93.184.216.2:22556"
	pm.mu.Lock()
	pm.sessions[srcAddr] = &peerLink{addr: srcAddr, mw: NewMsgWriter(srcC, magic)}
	pm.sessions[relayAddr] = &peerLink{addr: relayAddr, mw: NewMsgWriter(relayC, magic)}
	pm.mu.Unlock()

	pool := mempool.New(100)
	prev := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000, PkScript: []byte{0x51}}},
	}
	prevRaw, _ := prev.Serialize()
	_ = pool.Add(prevRaw)
	child := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prev.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 9_000_000, PkScript: []byte{0x51}}},
	}
	childRaw, _ := child.Serialize()

	done := make(chan struct{})
	go func() {
		pm.BroadcastTx(childRaw, srcAddr, pool, nil, nil)
		close(done)
	}()

	_ = srcS.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if cmd, _, err := wire.ReadMessage(srcS, magic); err == nil {
		t.Fatalf("source peer got %q, want no message", cmd)
	}
	_ = relayS.SetReadDeadline(time.Now().Add(2 * time.Second))
	cmd, _, err := wire.ReadMessage(relayS, magic)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "inv" {
		t.Fatalf("relay cmd %q want inv", cmd)
	}
	<-done
}
