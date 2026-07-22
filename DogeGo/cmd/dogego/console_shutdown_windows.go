//go:build windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
)

const (
	ctrlCEvent       = 0
	ctrlBreakEvent   = 1
	ctrlCloseEvent   = 2
	ctrlLogoffEvent  = 5
	ctrlShutdownEvent = 6
)

var consoleShutdownOnce atomic.Uint32

// installConsoleGracefulShutdown maps closing the console window (X) and Ctrl+Break
// to the same cancel used for Ctrl+C, so defers in node.Run can flush chain data.
func installConsoleGracefulShutdown(cancel context.CancelFunc) {
	if cancel == nil {
		return
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleCtrlHandler := kernel32.NewProc("SetConsoleCtrlHandler")
	cb := syscall.NewCallback(func(ctrlType uint32) uintptr {
		switch ctrlType {
		case ctrlCEvent, ctrlBreakEvent, ctrlLogoffEvent, ctrlShutdownEvent:
			if !consoleShutdownOnce.CompareAndSwap(0, 1) {
				return 1
			}
			if ctrlType == ctrlCloseEvent {
				_, _ = fmt.Fprintln(os.Stderr, "DogeGo: console closing - saving mempool/UTXO and stopping (do not force-kill in Task Manager)")
			}
			cancel()
			return 1
		case ctrlCloseEvent:
			if trayMinimizeOnClose.Load() {
				hideConsoleWindow()
				return 1
			}
			if !consoleShutdownOnce.CompareAndSwap(0, 1) {
				return 1
			}
			_, _ = fmt.Fprintln(os.Stderr, "DogeGo: console closing - saving mempool/UTXO and stopping (do not force-kill in Task Manager)")
			cancel()
			return 1
		default:
			return 0
		}
	})
	_, _, err := setConsoleCtrlHandler.Call(cb, 1)
	if err != syscall.Errno(0) {
		_, _ = fmt.Fprintf(os.Stderr, "DogeGo: console shutdown hook: %v\n", err)
	}
}
