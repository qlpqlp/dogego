// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
	"time"

	"dogego/rpc"
)

func TestBuildSoloPrimaryPeerInfoRowCoreFields(t *testing.T) {
	mw := NewMsgWriter(nil, [4]byte{0xc0, 0xc0, 0xc0, 0xc0})
	AttachWriterMsgStats(mw)
	mw.msgStats.addSent("ping", 100)
	mw.msgStats.addRecv("pong", 80)
	mb := NewMisbehaviorTracker(rpc.NewMemoryBanManager())
	mb.Note("1.2.3.4:22556", 10, "test")
	now := time.Now()
	row := BuildSoloPrimaryPeerInfoRow(SoloPrimaryPeerInfoOpts{
		Addr: "1.2.3.4:22556", ConnTime: now, ProtocolVersion: 70015, SubVer: "/DogeGo/",
		ServicesHex: "0000000000000009", TipH: 100, SyncedBlocks: 50, RelayTxes: true,
		MsgWriter: mw, Misbehavior: mb, Note: "solo",
	})
	if row["relaytxes"] != true {
		t.Fatalf("relaytxes %#v", row["relaytxes"])
	}
	if row["restricted"] != false {
		t.Fatalf("restricted %#v", row["restricted"])
	}
	if row["banscore"].(int) != 10 {
		t.Fatalf("banscore %#v", row["banscore"])
	}
	sent, _ := row["bytessent_per_msg"].(map[string]int64)
	if sent["ping"] != 100 {
		t.Fatalf("bytessent_per_msg %#v", row["bytessent_per_msg"])
	}
}
