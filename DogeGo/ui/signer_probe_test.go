// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dogego/config"
)

func TestApplySignerCmdFormOverride(t *testing.T) {
	conf := config.File{SignerCmd: "old cmd"}
	conf = ApplySignerCmdFormOverride(conf, "python hwi.py --stdin")
	if conf.SignerCmd != "python hwi.py --stdin" {
		t.Fatalf("override: %q", conf.SignerCmd)
	}
	conf = ApplySignerCmdFormOverride(conf, "")
	if conf.SignerCmd != "python hwi.py --stdin" {
		t.Fatalf("empty override should keep cmd: %q", conf.SignerCmd)
	}
}

func TestProbeSignerTestEmpty(t *testing.T) {
	res := ProbeSignerTest(config.File{}, "")
	if res.OK || res.SignerConfigured || len(res.Errors) == 0 {
		t.Fatalf("expected empty cmd error: %+v", res)
	}
}

func TestProbeSignerTestMock(t *testing.T) {
	mock := buildMockSignerForUI(t)
	res := ProbeSignerTest(config.File{}, mock)
	if !res.OK || !res.SignerConfigured || res.DeviceCount != 1 {
		t.Fatalf("mock probe: %+v", res)
	}
}

func buildMockSignerForUI(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "mocksigner")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	root := uiModuleRoot(t)
	cmd := exec.Command("go", "build", "-o", out, "./signer/cmd/mocksigner")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build mocksigner: %v\n%s", err, b)
	}
	return out
}

func uiModuleRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "env", "GOMOD")
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	mod := strings.TrimSpace(string(b))
	if mod == "" || mod == "/dev/null" {
		t.Fatal("GOMOD empty")
	}
	return filepath.Dir(mod)
}
