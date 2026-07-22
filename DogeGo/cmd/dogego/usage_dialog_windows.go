//go:build windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"syscall"
	"unsafe"
)

// showUsageDialog explains that DogeGo is a CLI tool (double-click looks like a crash).
func showUsageDialog() {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBoxW := user32.NewProc("MessageBoxW")
	title, err := syscall.UTF16PtrFromString("DogeGo")
	if err != nil {
		return
	}
	const text = "DogeGo is a command-line program - it is not meant to be double-clicked.\r\n\r\n" +
		"Open Command Prompt or PowerShell, change to this folder, then run for example:\r\n\r\n" +
		"  dogego.exe genesis\r\n\r\n" +
		"  dogego.exe node\r\n" +
		"    (default full node: headers + raw block payloads; without -datadir: setup wizard or dogecoinconf.json)\r\n\r\n" +
		"  dogego.exe spvnode\r\n" +
		"    (default SPV / headers-only; same flags as node; or use node -mode spv)\r\n\r\n" +
		"  dogego.exe node -datadir .\\dogedata [-peer HOST:PORT]\r\n\r\n" +
		"  dogego.exe ping -host 127.0.0.1\r\n\r\n" +
		"  dogego.exe indexer\r\n" +
		"    (prints help + chain status; use init | scan | status)\r\n\r\n" +
		"With \"node\" or \"spvnode\", the dashboard is at http://localhost:2013/ unless you use -nowebui.\r\n" +
		"Without -datadir, \"node\" opens the setup wizard in your browser."
	body, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	const mbOK = 0x0
	const mbIconInformation = 0x40
	_, _, _ = messageBoxW.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), mbOK|mbIconInformation)
}
