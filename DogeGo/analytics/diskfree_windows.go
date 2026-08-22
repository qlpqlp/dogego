// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

//go:build windows

package analytics

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	modkernel32Disk         = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceExW = modkernel32Disk.NewProc("GetDiskFreeSpaceExW")
)

func volumeUsage(path string) (free, total uint64, err error) {
	path = filepath.Clean(path)
	if path == "" {
		return 0, 0, fmt.Errorf("empty path")
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	r1, _, e1 := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if r1 == 0 {
		if e1 != syscall.Errno(0) {
			return 0, 0, e1
		}
		return 0, 0, fmt.Errorf("GetDiskFreeSpaceEx failed")
	}
	return freeBytesAvailable, totalBytes, nil
}
