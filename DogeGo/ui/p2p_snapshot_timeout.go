// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "time"

const p2pSnapshotTimeout = 5 * time.Second

// p2PSnapshotWithTimeout runs cfg.P2PSnapshot on a goroutine and returns nil on timeout.
// During heavy IBD the full P2P snapshot can block on locks; the dashboard still renders journal/contiguous fields.
func p2PSnapshotWithTimeout(fn func() map[string]any) map[string]any {
	return p2PSnapshotWithTimeoutDur(fn, p2pSnapshotTimeout)
}

func p2PSnapshotWithTimeoutDur(fn func() map[string]any, d time.Duration) map[string]any {
	if fn == nil {
		return nil
	}
	if d <= 0 {
		d = p2pSnapshotTimeout
	}
	ch := make(chan map[string]any, 1)
	go func() {
		ch <- fn()
	}()
	select {
	case s := <-ch:
		return s
	case <-time.After(d):
		return nil
	}
}
