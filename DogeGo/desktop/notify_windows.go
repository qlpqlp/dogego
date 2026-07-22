//go:build windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"fmt"
	"os/exec"
)

func platformNotify(title, body string) {
	script := fmt.Sprintf(
		`Add-Type -AssemblyName System.Windows.Forms; $n=New-Object System.Windows.Forms.NotifyIcon; $n.Icon=[System.Drawing.SystemIcons]::Information; $n.Visible=$true; $n.ShowBalloonTip(12000,%q,%q,[System.Windows.Forms.ToolTipIcon]::Info); Start-Sleep -Seconds 13; $n.Dispose()`,
		title, body,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
}
