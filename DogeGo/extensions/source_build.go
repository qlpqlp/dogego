// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const sourceBuildTimeout = 120

// buildSubprocessIfNeeded compiles entry.binary when a subprocess extension ships source only.
func buildSubprocessIfNeeded(extDir string, man Manifest) error {
	if man.Entry.Type != EntrySubprocess {
		return nil
	}
	binName := strings.TrimSpace(man.Entry.Binary)
	if binName == "" {
		return fmt.Errorf("subprocess extension %q missing entry.binary", man.ID)
	}
	if subprocessBinaryExists(extDir, binName) {
		return nil
	}
	pkg, err := locateGoMainPackage(extDir, man)
	if err != nil {
		return fmt.Errorf("extension %q: no prebuilt binary and %w (install Go or ship a binary in the zip)", man.ID, err)
	}
	outPath := filepath.Join(extDir, binName)
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(outPath), ".exe") {
		outPath += ".exe"
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("extension %q: go not found on PATH", man.ID)
	}
	cmd := exec.Command(goBin, "build", "-ldflags=-s -w", "-trimpath", "-o", outPath, pkg)
	cmd.Dir = extDir
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build %s: %w: %s", pkg, err, strings.TrimSpace(string(out)))
	}
	if !subprocessBinaryExists(extDir, binName) {
		return fmt.Errorf("go build finished but binary %q missing", binName)
	}
	return nil
}

func subprocessBinaryExists(extDir, binName string) bool {
	base := filepath.Join(extDir, binName)
	if st, err := os.Stat(base); err == nil && !st.IsDir() {
		return true
	}
	if runtime.GOOS == "windows" {
		if st, err := os.Stat(base + ".exe"); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func locateGoMainPackage(extDir string, man Manifest) (string, error) {
	candidates := []string{}
	if seg := extensionIDTail(man.ID); seg != "" {
		candidates = append(candidates, "./"+seg)
	}
	candidates = append(candidates, "./hello", ".")
	seen := make(map[string]struct{})
	for _, rel := range candidates {
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		dir := extDir
		if rel != "." {
			dir = filepath.Join(extDir, filepath.FromSlash(strings.TrimPrefix(rel, "./")))
		}
		if hasGoMainPackage(dir) {
			return rel, nil
		}
	}
	return "", fmt.Errorf("no Go main package found under %s", extDir)
}

func extensionIDTail(id string) string {
	id = strings.TrimSpace(id)
	if i := strings.LastIndex(id, "."); i >= 0 && i+1 < len(id) {
		return id[i+1:]
	}
	return ""
}

func hasGoMainPackage(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			continue
		}
		if strings.Contains(string(raw), "package main") {
			return true
		}
	}
	return false
}
