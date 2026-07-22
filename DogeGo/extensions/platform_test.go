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
)

func TestSelectPlatformArtifact(t *testing.T) {
	downloads := map[string]PlatformArtifact{
		"windows-amd64": {DownloadURL: "https://example.invalid/hello-windows-amd64.zip"},
		"linux-amd64":   {DownloadURL: "https://example.invalid/hello-linux-amd64.zip"},
		"universal":     {DownloadURL: "https://example.invalid/hello-universal.zip"},
	}
	key, art, err := SelectPlatformArtifact(downloads)
	if err != nil {
		t.Fatal(err)
	}
	want := CurrentPlatformKey()
	if key != want && key != "universal" {
		t.Fatalf("key %q art %q", key, art.DownloadURL)
	}
	if art.DownloadURL == "" {
		t.Fatal("empty url")
	}
}

func TestMaterializePlatformBinary(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "bin", CurrentPlatformKey())
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcName := "hello-ext"
	if runtime.GOOS == "windows" {
		srcName += ".exe"
	}
	srcPath := filepath.Join(srcDir, srcName)
	if err := os.WriteFile(srcPath, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := filepath.ToSlash(filepath.Join("bin", CurrentPlatformKey(), srcName))
	man := Manifest{
		ID: "test.bin", Name: "t", Version: "1",
		Entry: Entry{
			Type:   EntrySubprocess,
			Module: "test.bin",
			Binary: "hello-ext",
			Binaries: map[string]string{
				CurrentPlatformKey(): rel,
			},
		},
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := materializePlatformBinary(dir, man); err != nil {
		t.Fatal(err)
	}
	if !subprocessBinaryExists(dir, "hello-ext") {
		t.Fatal("expected materialized binary")
	}
}

func TestValidateManifestPlatformBinaries(t *testing.T) {
	m := Manifest{
		ManifestVersion: 1,
		ID:              "com.example.bin",
		Name:            "Bin",
		Version:         "1.0.0",
		Entry: Entry{
			Type:     EntrySubprocess,
			Module:   "com.example.bin",
			Binary:   "plugin",
			Binaries: map[string]string{"windows-amd64": "bin/win/plugin.exe"},
		},
	}
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	m.Entry.Binaries = map[string]string{"bad key": "x"}
	if err := ValidateManifest(m); err == nil {
		t.Fatal("expected invalid platform key")
	}
}
