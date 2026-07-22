// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

//go:build !windows && !linux

package node

// systemFreeMemoryMB is unavailable on this platform (falls back to DefaultDBCacheMB).
func systemFreeMemoryMB() int64 { return -1 }
