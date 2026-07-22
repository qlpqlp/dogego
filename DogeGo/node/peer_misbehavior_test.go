// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"

	"dogego/rpc"
)

func TestMisbehaviorThresholdBans(t *testing.T) {
	ban := rpc.NewMemoryBanManager()
	m := NewMisbehaviorTracker(ban)
	m.Note("1.2.3.4:22556", 50, "test")
	if ban.IsBanned(net.ParseIP("1.2.3.4")) {
		t.Fatal("should not ban yet")
	}
	m.Note("1.2.3.4:22556", 50, "test2")
	if !ban.IsBanned(net.ParseIP("1.2.3.4")) {
		t.Fatal("should ban at 100")
	}
}
