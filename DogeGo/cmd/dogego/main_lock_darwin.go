//go:build darwin && cgo

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import "runtime"

func init() {
	// Keep main.main on the process main OS thread for the life of the process.
	// fyne.io/systray's Darwin nativeLoop (NSApplication / NSStatusItem) SIGTRAPs if
	// started from any other thread - even one locked with runtime.LockOSThread().
	runtime.LockOSThread()
}
