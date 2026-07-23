// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"fmt"
	"path/filepath"
	"strings"
)

// sanitizeZipEntryRel normalizes a zip entry path relative to the package root.
// PowerShell Compress-Archive sometimes stores sibling files as "../icon.png";
// strip leading "../" / "..\" so those still land in the package root safely.
func sanitizeZipEntryRel(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, `\`, "/")
	name = strings.TrimPrefix(name, "/")
	for strings.HasPrefix(name, "../") {
		name = strings.TrimPrefix(name, "../")
	}
	name = strings.TrimPrefix(name, "./")
	if name == "" || name == "." {
		return "", fmt.Errorf("empty zip entry path")
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("zip path traversal: %q", name)
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("zip path absolute: %q", name)
	}
	return clean, nil
}
