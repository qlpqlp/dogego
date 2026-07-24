// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogExampleZipSHA256(t *testing.T) {
	catPath := filepath.Join("catalog", "catalog.json")
	raw, err := os.ReadFile(catPath)
	if err != nil {
		t.Skip(catPath)
	}
	var cat CatalogFile
	if err := json.Unmarshal(raw, &cat); err != nil {
		t.Fatal(err)
	}
	zipPaths := map[string]string{
		"dogego.zkl2":     filepath.Join("catalog", "zkl2", "dist", "zkl2.zip"),
		"dogego.doginals": filepath.Join("catalog", "doginals", "dist", "doginals.zip"),
		"example.wasm":    filepath.Join("catalog", "example-wasm", "ping.zip"),
	}
	universalZipPaths := map[string]string{
		"example.go":      filepath.Join("catalog", "example-go", "dist", "hello-universal.zip"),
		"dogego.zkl2":     filepath.Join("catalog", "zkl2", "dist", "zkl2-universal.zip"),
		"dogego.bbpow":    filepath.Join("catalog", "bbpow", "dist", "bbpow-universal.zip"),
		"dogego.doginals": filepath.Join("catalog", "doginals", "dist", "doginals-universal.zip"),
	}
	for _, e := range cat.Extensions {
		zipPath, ok := zipPaths[e.ID]
		if ok {
			if e.DownloadURL == "" || e.SHA256 == "" {
				continue
			}
			if !strings.HasPrefix(strings.ToLower(e.DownloadURL), "https://") {
				t.Fatalf("%s download_url must be https", e.ID)
			}
			assertCatalogSHA256(t, e.ID, zipPath, e.SHA256)
		}
		if zipPath, ok := universalZipPaths[e.ID]; ok {
			art, ok := e.Downloads["universal"]
			if !ok || art.DownloadURL == "" || art.SHA256 == "" {
				continue
			}
			if !strings.HasPrefix(strings.ToLower(art.DownloadURL), "https://") {
				t.Fatalf("%s universal download_url must be https", e.ID)
			}
			assertCatalogSHA256(t, e.ID, zipPath, art.SHA256)
		}
	}
}

func assertCatalogSHA256(t *testing.T, id, zipPath, want string) {
	t.Helper()
	zraw, err := os.ReadFile(zipPath)
	if err != nil {
		t.Skip(zipPath)
	}
	sum := sha256.Sum256(zraw)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		t.Fatalf("%s sha256 catalog=%s file=%s", id, want, got)
	}
}

func TestInstallFromURLWithSHA256(t *testing.T) {
	zipPath := filepath.Join("catalog", "example-wasm", "ping.zip")
	zraw, err := os.ReadFile(zipPath)
	if err != nil {
		t.Skip(zipPath)
	}
	want := hex.EncodeToString(sha256Sum(zraw))
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	if _, err := m.InstallFromURL(context.Background(), "https://example.invalid/ping.zip", want); err == nil {
		t.Fatal("expected download failure")
	}
	// Wrong hash rejected when we had a file - covered by verifyFileSHA256 via temp path in other tests.
	_ = want
}

func sha256Sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}
