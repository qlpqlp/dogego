// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"strings"

	"dogego/extensions/catalog"
)

// Legacy docs aliases for ids that do not match an embedded package folder.
var legacyDocsPaths = map[string]string{
	"example.hello": "extensions/catalog/HELLO_WORLD.md",
}

// NormalizeDocsPath maps legacy docs/extensions paths to extensions/catalog.
func NormalizeDocsPath(docsPath string) string {
	p := strings.TrimSpace(docsPath)
	p = strings.ReplaceAll(p, "docs/extensions/", "extensions/catalog/")
	return p
}

// EnrichDocsPath returns docsPath when set, otherwise the package manifest docs_path
// (auto-discovered from embedded/disk catalog packages), then legacy aliases.
func EnrichDocsPath(id, docsPath string) string {
	if p := NormalizeDocsPath(docsPath); p != "" {
		return p
	}
	id = strings.TrimSpace(id)
	if p := NormalizeDocsPath(catalog.PackageDocsPath(id)); p != "" {
		return p
	}
	return legacyDocsPaths[id]
}
