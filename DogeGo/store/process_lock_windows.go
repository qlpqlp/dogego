//go:build windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modKernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modKernel32.NewProc("LockFileEx")
	procUnlockFileEx = modKernel32.NewProc("UnlockFileEx")
)

const (
	lockFileFailImmediately = 0x00000001
	lockFileExclusiveLock   = 0x00000002
)

func lockProcessFile(f *os.File) error {
	h := syscall.Handle(f.Fd())
	var ol syscall.Overlapped
	r1, _, err := procLockFileEx.Call(
		uintptr(h),
		uintptr(lockFileFailImmediately|lockFileExclusiveLock),
		0,
		^uintptr(0),
		^uintptr(0),
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		if err == syscall.Errno(0) {
			return syscall.EWOULDBLOCK
		}
		return err
	}
	return nil
}

func unlockProcessFile(f *os.File) {
	h := syscall.Handle(f.Fd())
	var ol syscall.Overlapped
	_, _, _ = procUnlockFileEx.Call(
		uintptr(h),
		0,
		^uintptr(0),
		^uintptr(0),
		uintptr(unsafe.Pointer(&ol)),
	)
}
