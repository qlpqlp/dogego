//go:build unix

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"os"
	"syscall"
)

// platformShutdownSignals are OS signals that should trigger graceful node shutdown.
// SIGHUP: closing Terminal/iTerm or SSH disconnect; SIGTERM: docker stop, systemctl, kill.
func platformShutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}
