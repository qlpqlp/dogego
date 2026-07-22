//go:build windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

var (
	procOpenProcess = modKernel32.NewProc("OpenProcess")
	procCloseHandle = modKernel32.NewProc("CloseHandle")
)

const processQueryLimitedInformation = 0x1000

func processPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, _, _ := procOpenProcess.Call(
		uintptr(processQueryLimitedInformation),
		0,
		uintptr(pid),
	)
	if h == 0 {
		return false
	}
	_, _, _ = procCloseHandle.Call(h)
	return true
}
