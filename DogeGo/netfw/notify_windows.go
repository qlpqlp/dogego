//go:build windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package netfw

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	mbYesNo           = 0x00000004
	mbIconWarning     = 0x00000030
	mbIconInformation = 0x00000040
	idYes             = 6
)

func ConfirmElevation(cfg Config) bool {
	title := "DogeGo - Firewall"
	body := fmt.Sprintf(
		"DogeGo needs Administrator permission to add Windows Firewall rules for P2P (TCP port %d and outbound %s).\r\n\r\n"+
			"Click Yes to show the Windows permission prompt (same as many other apps).\r\nClick No to skip - you can paste the commands from the DogeGo dashboard later.",
		cfg.Port, cfg.ExePath)
	return messageBox(title, body, mbYesNo|mbIconInformation) == idYes
}

func NotifyFirewallSetupNeeded(cfg Config, res Result) {
	if res.OK || res.AlreadyOK || cfg.Mode == ModeNever {
		return
	}
	title := "DogeGo - Firewall rules needed"
	var b strings.Builder
	if res.UserMessage != "" {
		b.WriteString(res.UserMessage)
		b.WriteString("\r\n\r\n")
	}
	if !Present(cfg) {
		b.WriteString("P2P sync may fail or peers may disconnect until these rules exist.\r\n\r\n")
		b.WriteString("Run in an elevated Command Prompt or PowerShell:\r\n\r\n")
		for _, cmd := range ManualCommands(cfg) {
			b.WriteString("  ")
			b.WriteString(cmd)
			b.WriteString("\r\n")
		}
		b.WriteString("\r\n(Also shown on the Overview tab in the web dashboard.)")
	}
	messageBox(title, b.String(), mbIconWarning)
}

func thirdPartyFirewallNotePlatform() string {
	return "Windows Defender Firewall is separate from third-party tools (Norton, McAfee, etc.) - allow dogego.exe and the P2P port in those products too if P2P still fails after adding the rules below."
}

func messageBox(title, body string, flags uintptr) int {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBoxW := user32.NewProc("MessageBoxW")
	tPtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}
	bPtr, err := syscall.UTF16PtrFromString(body)
	if err != nil {
		return 0
	}
	r, _, _ := messageBoxW.Call(0, uintptr(unsafe.Pointer(bPtr)), uintptr(unsafe.Pointer(tPtr)), flags)
	return int(r)
}
