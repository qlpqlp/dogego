// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"
)

func TestMaintainerTickIntervalIBD(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{MaxOutbound: 4}, mustTestnetParams(t), "/DogeGo/", net.Dialer{})
	raw := &progressiveRawState{}
	raw.SetSyncParallelism(2)
	pm.SetRelayEnv(RelayEnv{RawFill: raw})
	if pm.maintainerTickInterval() != peerMaintainerIntervalIBD {
		t.Fatalf("want IBD interval, got %v", pm.maintainerTickInterval())
	}
	raw.mu.Lock()
	raw.idleFull = true
	raw.mu.Unlock()
	if pm.maintainerTickInterval() != peerMaintainerIntervalNormal {
		t.Fatalf("want normal interval when caught up")
	}
}
