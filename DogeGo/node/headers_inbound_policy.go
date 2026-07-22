// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "strings"

// shouldTryNextHeaderSyncPeer reports whether header sync should continue with another peer or getheaders round.
func shouldTryNextHeaderSyncPeer(err error) bool {
	if err == nil {
		return false
	}
	if IsHeaderRewindRetryErr(err) {
		return true
	}
	return recoverableHeaderPeerErr(err)
}

// InboundHeadersErrorPolicy classifies ApplyHeadersMessage failures for steady-state P2P handling.
// Local journal rewinds must not increment peer misbehavior; transport/peer-chain errors should try recovery.
func InboundHeadersErrorPolicy(err error) (retryTopUp bool, pausePrimary bool, misbehavior bool) {
	if err == nil {
		return false, false, false
	}
	if IsHeaderRewindRetryErr(err) {
		return true, false, false
	}
	if strings.Contains(err.Error(), "fork deferred (marginal chain work") {
		return false, false, false
	}
	if recoverableHeaderPeerErr(err) {
		return false, true, false
	}
	return false, false, true
}
