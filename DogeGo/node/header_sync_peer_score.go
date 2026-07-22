// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "strings"

// headerSyncFailureHard reports whether a failed header-sync session should use a long peer cooldown.
func headerSyncFailureHard(err error) bool {
	if err == nil || IsBenignShutdownErr(err) || IsHeaderRewindRetryErr(err) {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "header sync stall") ||
		strings.Contains(s, "timeout waiting for headers") ||
		strings.Contains(s, "header sync incomplete") ||
		strings.Contains(s, "bad nBits") ||
		strings.Contains(s, "checkpoint hash mismatch")
}

// noteHeaderSyncPeerFailure records header-sync outcome on the block peer scorer and addrbook (Core: disconnect slow header peers).
func noteHeaderSyncPeerFailure(scorer *BlockPeerScorer, book *AddrBook, addr string, err error) {
	if addr == "" || err == nil {
		return
	}
	if !shouldTryNextHeaderSyncPeer(err) || IsHeaderRewindRetryErr(err) {
		return
	}
	penalizeBlockPeer(scorer, book, addr, headerSyncFailureHard(err))
}
