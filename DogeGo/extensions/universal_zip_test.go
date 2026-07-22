// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"path/filepath"
	"testing"
)

func TestInstallHelloUniversalZip(t *testing.T) {
	zipPath := filepath.Join("catalog", "example-go", "dist", "hello-universal.zip")
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	row, err := m.InstallZip(zipPath)
	if err != nil {
		t.Fatalf("InstallZip: %v", err)
	}
	if row.ID != "example.go" {
		t.Fatalf("id %q", row.ID)
	}
	extDir := filepath.Join(dir, "extensions", "example.go")
	if !subprocessBinaryExists(extDir, "hello-ext") {
		t.Fatalf("expected materialized hello-ext (platform %s)", CurrentPlatformKey())
	}
}
