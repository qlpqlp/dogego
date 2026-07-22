//go:build !windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import "context"

// installConsoleGracefulShutdown is a no-op off Windows (Unix uses SIGHUP/SIGTERM via platformShutdownSignals).
func installConsoleGracefulShutdown(cancel context.CancelFunc) {}
