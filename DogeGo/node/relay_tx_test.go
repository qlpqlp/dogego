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

	"dogego/consensus"
	"dogego/mempool"
	"dogego/wire"
)

func TestRelayTxToPeerSendsInv(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	magic := [4]byte{0xc0, 0xc0, 0xc0, 0xc0}
	mw := NewMsgWriter(c1, magic)
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
	done := make(chan error, 1)
	go func() {
		done <- RelayTxToPeer(mw, childRaw, 0, pool, nil, nil)
	}()
	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cmd, pl, err := wire.ReadMessage(c2, magic)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if cmd != "inv" {
		t.Fatalf("cmd %q want inv", cmd)
	}
	entries, err := wire.DecodeInvPayload(pl)
	if err != nil || len(entries) != 1 || entries[0].Type != wire.InvTypeTx {
		t.Fatalf("inv entries: %v err %v", entries, err)
	}
}

func TestRelayTxToPeerSkipsBelowFeeFilter(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	magic := [4]byte{0xc0, 0xc0, 0xc0, 0xc0}
	mw := NewMsgWriter(c1, magic)
	pool := mempool.New(100)
	prev := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000, PkScript: []byte{0x51}}},
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
		Vout: []wire.TxOut{{Value: 900_000, PkScript: []byte{0x51}}},
	}
	childRaw, _ := child.Serialize()
	view := consensus.AdmissionPrevOutView(pool, nil, nil)
	rate, ok := consensus.TxFeeRateKoinuPerKB(child, childRaw, view)
	if !ok {
		t.Fatal("fee rate")
	}
	highFilter := rate + 1_000_000
	go func() { _ = RelayTxToPeer(mw, childRaw, highFilter, pool, nil, nil) }()
	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err := wire.ReadMessage(c2, magic)
	if err == nil {
		t.Fatal("expected no relay below feefilter")
	}
}
