// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/rpc"
	"dogego/wire"
)

func TestRejectWitnessTxIfPresent(t *testing.T) {
	tx := &wire.Tx{Version: 1, LockTime: 0}
	tx.Vin = []wire.TxIn{{}}
	tx.Vout = []wire.TxOut{{Value: 1, PkScript: []byte{0x76, 0xa9, 0x14}}}
	tx.Vin[0].Witness = [][]byte{{0x01}}
	if !RejectWitnessTxIfPresent(nil, "203.0.113.1:22556", nil, tx) {
		t.Fatal("expected witness rejection")
	}
	tx.Vin[0].Witness = nil
	if RejectWitnessTxIfPresent(nil, "203.0.113.1:22556", nil, tx) {
		t.Fatal("legacy tx should pass witness gate")
	}
}

func TestRejectWitnessTxMisbehaviorScore(t *testing.T) {
	tx := &wire.Tx{Version: 1}
	tx.Vin = []wire.TxIn{{Witness: [][]byte{{0x00}}}}
	tx.Vout = []wire.TxOut{{Value: 0}}
	mb := NewMisbehaviorTracker(rpc.NewMemoryBanManager())
	_ = RejectWitnessTxIfPresent(nil, "203.0.113.2:22556", mb, tx)
	mb.mu.Lock()
	score := mb.scores["203.0.113.2"]
	mb.mu.Unlock()
	if score != misbehaviorWitnessTx {
		t.Fatalf("score %d want %d", score, misbehaviorWitnessTx)
	}
}
