// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package fieldevidence

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"dogego/offlinegate"
)

// RunOffline executes bootstrap + field-evidence suites from module root.
func RunOffline(root string, stdout, stderr io.Writer) error {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	if err := offlinegate.RunBootstrap(root, stdout, stderr); err != nil {
		return fmt.Errorf("bootstrap consensus/testdata: %w", err)
	}
	for _, s := range DefaultSuites() {
		cmd := exec.Command("go", s.Args...)
		cmd.Dir = root
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
	}
	return nil
}
