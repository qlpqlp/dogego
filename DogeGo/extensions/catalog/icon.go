// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package catalog

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ReadIconBytes loads a PNG from a catalog package folder (embed, then disk).
func ReadIconBytes(catalogDir, rel string) ([]byte, error) {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, fs.ErrNotExist
	}
	path := filepath.ToSlash(filepath.Join(catalogDir, rel))
	if b, err := Files.ReadFile(path); err == nil {
		return b, nil
	}
	if root, ok := moduleRootDir(); ok {
		full := filepath.Join(root, "extensions", "catalog", filepath.FromSlash(path))
		return os.ReadFile(full)
	}
	return nil, fs.ErrNotExist
}
