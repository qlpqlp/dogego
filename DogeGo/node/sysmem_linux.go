// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

//go:build linux

package node

import "syscall"

// systemFreeMemoryMB returns currently available physical RAM in megabytes, or -1 on failure.
func systemFreeMemoryMB() int64 {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return -1
	}
	unit := uint64(info.Unit)
	if unit == 0 {
		unit = 1
	}
	free := uint64(info.Freeram) * unit
	return int64(free / (1024 * 1024))
}
