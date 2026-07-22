// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "sync/atomic"

// ibdExitLatch mirrors Core IsInitialBlockDownload's latchToFalse: once the node leaves IBD it
// stays out of IBD until process restart (even if tip age or chain work later regress).
var ibdExitLatch atomic.Bool

// ResetIBDExitLatchForTests clears the latch (tests only).
func ResetIBDExitLatchForTests() {
	ibdExitLatch.Store(false)
}

// ResetIBDExitLatch clears the Core-style IBD latch after a chain rewind so RPC reflects catch-up again.
func ResetIBDExitLatch() {
	ibdExitLatch.Store(false)
}

func applyIBDExitLatch(ibd bool) bool {
	if ibdExitLatch.Load() {
		return false
	}
	if !ibd {
		ibdExitLatch.Store(true)
	}
	return ibd
}
