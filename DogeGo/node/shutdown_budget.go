// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package node

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// GracefulShutdownBudget is how long defers may flush before the process is force-exited.
const GracefulShutdownBudget = 20 * time.Second

// ForceExitAfterGrace is a small cushion after the flush budget before os.Exit.
const ForceExitAfterGrace = 5 * time.Second

// ShutdownFlushBudget caps individual flush steps (UTXO snapshot, mempool, etc.).
const ShutdownFlushBudget = 12 * time.Second

// WrapStopWithForceExit returns a Stop callback that cancels ctx once and force-exits
// if the process is still alive after GracefulShutdownBudget + ForceExitAfterGrace.
func WrapStopWithForceExit(cancel func()) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			fmt.Fprintln(os.Stderr, "DogeGo: shutting down (graceful budget "+GracefulShutdownBudget.String()+")…")
			if cancel != nil {
				cancel()
			}
			go func() {
				time.Sleep(GracefulShutdownBudget + ForceExitAfterGrace)
				fmt.Fprintln(os.Stderr, "DogeGo: graceful shutdown timed out; forcing process exit (databases will be repaired on next start if needed)")
				os.Exit(1)
			}()
		})
	}
}

// RunWithTimeout runs fn and returns false if it does not finish within d.
func RunWithTimeout(d time.Duration, fn func()) bool {
	if fn == nil {
		return true
	}
	if d <= 0 {
		fn()
		return true
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}
