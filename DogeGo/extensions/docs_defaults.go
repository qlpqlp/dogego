// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import "strings"

// Builtin docs paths ship with DogeGo (embedded markdown under extensions/catalog/).
var builtinDocsPaths = map[string]string{
	"dogego.zkl2":     "extensions/catalog/zkl2/docs/USER_GUIDE.md",
	"dogego.doginals": "extensions/catalog/doginals/docs/USER_GUIDE.md",
	"example.go":      "extensions/catalog/example-go/docs/README.md",
	"example.hello":   "extensions/catalog/HELLO_WORLD.md", // legacy id
	"example.wasm":    "extensions/catalog/example-wasm/docs/README.md",
}

// NormalizeDocsPath maps legacy docs/extensions paths to extensions/catalog.
func NormalizeDocsPath(docsPath string) string {
	p := strings.TrimSpace(docsPath)
	p = strings.ReplaceAll(p, "docs/extensions/", "extensions/catalog/")
	return p
}

// EnrichDocsPath returns docsPath when set, otherwise a built-in default for known extensions.
func EnrichDocsPath(id, docsPath string) string {
	if p := NormalizeDocsPath(docsPath); p != "" {
		return p
	}
	return builtinDocsPaths[strings.TrimSpace(id)]
}
