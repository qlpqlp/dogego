// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"net"
	"testing"
	"time"

	"dogego/consensus"
	"dogego/rpc"
	"dogego/wire"
)

func TestHandleInboundTxAdmissionFailureSkipsOrphan(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	magic := [4]byte{0xc0, 0xc0, 0xc0, 0xc0}
	mw := NewMsgWriter(c1, magic)
	done := make(chan struct{})
	go func() {
		HandleInboundTxAdmissionFailure(nil, "1.2.3.4:22556", mw, nil, consensus.ErrOrphanTx)
		close(done)
	}()
	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err := wire.ReadMessage(c2, magic)
	<-done
	if err == nil {
		t.Fatal("expected no reject on wire")
	}
}

func TestHandleInboundTxAdmissionFailureRejectsMinFee(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	magic := [4]byte{0xc0, 0xc0, 0xc0, 0xc0}
	mw := NewMsgWriter(c1, magic)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	raw, _ := tx.Serialize()
	done := make(chan struct{})
	go func() {
		HandleInboundTxAdmissionFailure(raw, "1.2.3.4:22556", mw, NewMisbehaviorTracker(rpc.NewMemoryBanManager()), consensus.ErrMinRelayFee)
		close(done)
	}()
	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cmd, pl, err := wire.ReadMessage(c2, magic)
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "reject" {
		t.Fatalf("cmd %q", cmd)
	}
	rj, err := wire.DecodeRejectPayload(pl)
	if err != nil {
		t.Fatal(err)
	}
	if rj.Code != wire.RejectInsufficientFee {
		t.Fatalf("code 0x%02x", rj.Code)
	}
}

func TestHandleInboundTxAdmissionFailureSkipsDuplicate(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	magic := [4]byte{0xc0, 0xc0, 0xc0, 0xc0}
	mw := NewMsgWriter(c1, magic)
	done := make(chan struct{})
	go func() {
		HandleInboundTxAdmissionFailure(nil, "1.2.3.4:22556", mw, nil, fmt.Errorf("input 0: %w", consensus.ErrSpendInMempool))
		close(done)
	}()
	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err := wire.ReadMessage(c2, magic)
	<-done
	if err == nil {
		t.Fatal("duplicate spend should not trigger reject")
	}
}
