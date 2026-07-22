// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestStripsBOM(t *testing.T) {
	dir := t.TempDir()
	raw := []byte{0xEF, 0xBB, 0xBF}
	raw = append(raw, []byte(`{
  "manifest_version": 1,
  "id": "example.wasm",
  "name": "Wasm Ping",
  "version": "0.1.0",
  "entry": {"type": "wasm", "module": "example.wasm", "wasm": "ping.wasm"}
}`)...)
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "example.wasm" {
		t.Fatalf("id %q", m.ID)
	}
}
