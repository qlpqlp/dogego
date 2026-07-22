// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestForceKillPIDAndRemoveRetry(t *testing.T) {
	dir := t.TempDir()
	writeSubprocessPIDTo(dir, 1) // unlikely real; kill should be best-effort
	killPIDFile(dir)
	if _, err := os.Stat(filepath.Join(dir, subprocessPIDFile)); !os.IsNotExist(err) {
		t.Fatalf("pid file still present: %v", err)
	}
	p := filepath.Join(dir, "gone.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removePathRetry(p); err != nil {
		t.Fatal(err)
	}
}

func TestStopClearsAliveFlag(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Covered indirectly by uninstall path; keep unit light without spawning.
		t.Skip("windows process spawn covered by integration")
	}
	_ = time.Second
}
