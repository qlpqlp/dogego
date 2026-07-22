//go:build darwin

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
	script := fmt.Sprintf(`display notification %q with title %q`, body, title)
	cmd := exec.Command("osascript", "-e", script)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
}
