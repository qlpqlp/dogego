// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package signer

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseCommandLine(t *testing.T) {
	if ParseCommandLine("") != nil {
		t.Fatal("empty")
	}
	if ParseCommandLine("  ") != nil {
		t.Fatal("whitespace")
	}
	argv := ParseCommandLine("python hwi.py --chain dogecoin --stdin")
	if len(argv) != 5 || argv[0] != "python" || argv[4] != "--stdin" {
		t.Fatalf("argv=%v", argv)
	}
}

func TestClientAvailable(t *testing.T) {
	c, err := New(nil)
	if err != nil || c != nil {
		t.Fatalf("nil argv: c=%v err=%v", c, err)
	}
	c, err = New([]string{"echo"})
	if err != nil || !c.Available() {
		t.Fatalf("expected available: c=%v err=%v", c, err)
	}
}

func TestValidateCommandRejectsShellMetacharacters(t *testing.T) {
	if err := ValidateCommand([]string{"python;rm", "-rf", "/"}); err == nil {
		t.Fatal("expected reject")
	}
}

func TestClientCallTimeout(t *testing.T) {
	mock := buildMockSignerForSignerPkg(t)
	c, err := New([]string{mock, "--sleep=2s"})
	if err != nil || c == nil {
		t.Fatalf("new: %v", err)
	}
	c.Timeout = 200 * time.Millisecond
	_, err = c.Call("enumerate", nil)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("err=%v", err)
	}
}

func buildMockSignerForSignerPkg(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "mocksigner")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, "./cmd/mocksigner")
	cmd.Dir = "."
	// Run from signer package directory via module root.
	mod := exec.Command("go", "env", "GOMOD")
	b, err := mod.Output()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(strings.TrimSpace(string(b)))
	cmd.Dir = root
	cmd.Args = []string{"go", "build", "-o", out, "./signer/cmd/mocksigner"}
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if outb, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build mocksigner: %v\n%s", err, outb)
	}
	return out
}
