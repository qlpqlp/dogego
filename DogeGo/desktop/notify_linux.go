//go:build linux

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"os/exec"
)

func platformNotify(title, body string) {
	cmd := exec.Command("notify-send", "--app-name=DogeGo", title, body)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
}
