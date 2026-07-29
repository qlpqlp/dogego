// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package node

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// GracefulShutdownBudget is how long defers may flush before the process is force-exited.
const GracefulShutdownBudget = 20 * time.Second

// ForceExitAfterGrace is a small cushion after the flush budget before os.Exit.
const ForceExitAfterGrace = 5 * time.Second

// ShutdownFlushBudget caps individual flush steps (UTXO snapshot, mempool, etc.).
const ShutdownFlushBudget = 12 * time.Second

// intentionalStopClearDir is set when the node is ready so operator Stop() clears the unclean marker
// even if os.Exit skips defers (force-exit after the graceful budget).
var intentionalStopClearDir atomic.Value // string

// RegisterIntentionalStopClearDir records the chain data dir whose unclean-shutdown marker should be
// removed when the operator requests Stop (dashboard / tray / update apply).
func RegisterIntentionalStopClearDir(chainDir string) {
	chainDir = strings.TrimSpace(chainDir)
	if chainDir == "" {
		return
	}
	intentionalStopClearDir.Store(chainDir)
}

func clearIntentionalStopMarker() {
	v, ok := intentionalStopClearDir.Load().(string)
	if !ok || v == "" {
		return
	}
	ClearUncleanShutdown(v)
}

// WrapStopWithForceExit returns a Stop callback that cancels ctx once and force-exits
// if the process is still alive after GracefulShutdownBudget + ForceExitAfterGrace.
// Intentional Stop clears the unclean-shutdown marker so the next start does not run a
// synchronous hour-long repair after a dashboard/tray shutdown.
func WrapStopWithForceExit(cancel func()) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			clearIntentionalStopMarker()
			fmt.Fprintln(os.Stderr, "DogeGo: shutting down (graceful budget "+GracefulShutdownBudget.String()+")…")
			if cancel != nil {
				cancel()
			}
			go func() {
				time.Sleep(GracefulShutdownBudget + ForceExitAfterGrace)
				fmt.Fprintln(os.Stderr, "DogeGo: graceful shutdown timed out; forcing process exit")
				// os.Exit skips defers; clear again so force-exit after Stop is not treated as a crash.
				clearIntentionalStopMarker()
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
