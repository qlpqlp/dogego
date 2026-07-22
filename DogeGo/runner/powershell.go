// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ScriptRunResult reports one scripts/*.ps1 invocation.
type ScriptRunResult struct {
	Script  string `json:"script"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
}

// FindPowerShell returns pwsh or powershell when available.
func FindPowerShell() string {
	for _, name := range []string{"pwsh", "powershell"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// RunScript executes scripts/relScript under moduleRoot with PowerShell -File.
func RunScript(moduleRoot, relScript string, args []string, extraEnv map[string]string) ScriptRunResult {
	relScript = filepath.ToSlash(strings.TrimPrefix(relScript, "scripts/"))
	res := ScriptRunResult{Script: "scripts/" + relScript}
	ps := FindPowerShell()
	if ps == "" {
		res.Error = "powershell_not_found"
		return res
	}
	scriptPath := filepath.Join(moduleRoot, "scripts", filepath.FromSlash(relScript))
	if st, err := os.Stat(scriptPath); err != nil || st.IsDir() {
		res.Error = "script_missing"
		return res
	}
	cmdArgs := append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath}, args...)
	cmd := exec.Command(ps, cmdArgs...)
	cmd.Dir = moduleRoot
	cmd.Env = os.Environ()
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		res.Error = strings.TrimSpace(string(out))
		if res.Error == "" {
			res.Error = err.Error()
		}
		return res
	}
	res.OK = true
	return res
}

// RunGoTest runs go test with args in moduleRoot.
func RunGoTest(moduleRoot string, args ...string) error {
	if !hasGo() {
		return fmt.Errorf("go not in PATH")
	}
	cmd := exec.Command("go", append([]string{"test"}, args...)...)
	cmd.Dir = moduleRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
