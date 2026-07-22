// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package offlinegate

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

const bootstrapTest = "TestUpdateCoreTestdata"

// BootstrapArgs returns go test args for canonical consensus/testdata regeneration.
func BootstrapArgs() []string {
	return []string{"test", "./consensus", "-run", bootstrapTest, "-count=1"}
}

// BootstrapCommandLine returns the go test command string used in CI/cert scripts.
func BootstrapCommandLine() string {
	return "go " + strings.Join(BootstrapArgs(), " ")
}

// RunBootstrap regenerates consensus/testdata with UPDATE_CORE_TESTDATA=1.
func RunBootstrap(root string, stdout, stderr io.Writer) error {
	cmd := exec.Command("go", BootstrapArgs()...)
	cmd.Dir = root
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "UPDATE_CORE_TESTDATA=1")
	return cmd.Run()
}
