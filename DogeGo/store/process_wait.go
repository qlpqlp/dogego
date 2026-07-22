// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "time"

// WaitForProcessExit polls until pid is no longer running or maxWait elapses.
// Returns true when the process has exited.
func WaitForProcessExit(pid int, maxWait time.Duration) bool {
	if pid <= 0 {
		return true
	}
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if !processPIDAlive(pid) {
			return true
		}
		time.Sleep(120 * time.Millisecond)
	}
	return !processPIDAlive(pid)
}
