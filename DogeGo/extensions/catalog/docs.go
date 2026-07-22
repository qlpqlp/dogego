// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package catalog

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// PathPrefix is the canonical docs path prefix for catalog markdown.
const PathPrefix = "extensions/catalog/"

// Files holds the official extensions catalog (embedded at build time).
//
//go:embed catalog.json *.md example-go example-wasm zkl2 doginals
var Files embed.FS

// NormalizeDocPath maps legacy or absolute paths to a path relative to extensions/catalog/.
func NormalizeDocPath(rel string) string {
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "docs/")
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.ReplaceAll(rel, "extensions/catalog/", "")
	if strings.HasPrefix(rel, "extensions/catalog/") {
		rel = strings.TrimPrefix(rel, "extensions/catalog/")
	} else if strings.HasPrefix(rel, "extensions/") {
		rel = strings.TrimPrefix(rel, "extensions/")
		if strings.HasPrefix(rel, "catalog/") {
			rel = strings.TrimPrefix(rel, "catalog/")
		}
	}
	rel = path.Clean(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return ""
	}
	return rel
}

// IsCatalogMarkdownPath reports whether rel refers to catalog markdown.
func IsCatalogMarkdownPath(rel string) bool {
	low := strings.ToLower(strings.ReplaceAll(rel, "\\", "/"))
	if strings.Contains(low, "extensions/catalog/") || strings.Contains(low, "extensions/catalog/") {
		return strings.HasSuffix(low, ".md")
	}
	n := NormalizeDocPath(rel)
	if n == "" || !strings.HasSuffix(strings.ToLower(n), ".md") {
		return false
	}
	_, err := Files.ReadFile(n)
	return err == nil
}

// ReadMarkdown returns catalog markdown bytes and canonical path.
func ReadMarkdown(rel string) ([]byte, string, error) {
	n := NormalizeDocPath(rel)
	if n == "" {
		return nil, "", fmt.Errorf("invalid path")
	}
	if !strings.HasSuffix(strings.ToLower(n), ".md") {
		return nil, "", fmt.Errorf("not a markdown file")
	}
	if b, err := Files.ReadFile(n); err == nil {
		return b, PathPrefix + n, nil
	}
	if b, err := readDisk(n); err == nil {
		return b, PathPrefix + n, nil
	}
	return nil, "", fs.ErrNotExist
}

func readDisk(rel string) ([]byte, error) {
	root, ok := moduleRootDir()
	if !ok {
		return nil, fs.ErrNotExist
	}
	return os.ReadFile(filepath.Join(root, "extensions", "catalog", filepath.FromSlash(rel)))
}

func moduleRootDir() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	dir := wd
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}
