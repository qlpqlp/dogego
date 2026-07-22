// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"time"
)

const (
	peerMaintainerIntervalNormal = 40 * time.Second
	peerMaintainerIntervalIBD    = 12 * time.Second
)

// ibdBlockSyncActive is true while forward raw block catch-up is in progress (not caught up).
func (pm *PeerMgr) ibdBlockSyncActive() bool {
	if pm == nil {
		return false
	}
	raw := pm.relayEnv.RawFill
	if raw == nil {
		return false
	}
	return raw.useShortReadDeadline()
}

func (pm *PeerMgr) maintainerTickInterval() time.Duration {
	if pm.ibdBlockSyncActive() {
		return peerMaintainerIntervalIBD
	}
	return peerMaintainerIntervalNormal
}

func (pm *PeerMgr) maintainerOnce(ctx context.Context) {
	pm.EvictStaleRelayPeers()
	pm.probeFeeler(ctx)
	pm.tryDialMore(ctx)
	if pm.ibdBlockSyncActive() {
		pm.probeFeeler(ctx)
	}
}
