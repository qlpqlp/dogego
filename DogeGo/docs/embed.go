// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package docs

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"dogego/DIPs"
	extcatalog "dogego/extensions/catalog"
)

// Files holds operator markdown shipped with DogeGo (embedded at build time).
//
//go:embed *.md
var Files embed.FS

// ReadMarkdown returns file bytes for docs/NAME.md or NAME.md.
func ReadMarkdown(rel string) ([]byte, string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, "docs/")
	rel = path.Clean(rel)
	if rel == "." || rel == ".." || strings.Contains(rel, "..") {
		return nil, "", fmt.Errorf("invalid path")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".md") {
		return nil, "", fmt.Errorf("not a markdown file")
	}
	if dips.IsDIPMarkdownPath(rel) || dips.IsDIPMarkdownPath("DIPs/"+rel) {
		if b, name, err := dips.ReadMarkdown(rel); err == nil {
			return b, name, nil
		}
	}
	if extcatalog.IsCatalogMarkdownPath(rel) || extcatalog.IsCatalogMarkdownPath("docs/"+rel) {
		if b, name, err := extcatalog.ReadMarkdown(rel); err == nil {
			return b, name, nil
		}
	}
	b, err := Files.ReadFile(rel)
	if err == nil {
		return b, "docs/" + rel, nil
	}
	if _, ok := rootMarkdownAllow[rel]; ok {
		if rb, rerr := readRootMarkdown(rel); rerr == nil {
			return rb, rel, nil
		}
	}
	return nil, "", err
}

// MarkdownNames lists embedded markdown basenames as docs/ paths.
func MarkdownNames() ([]string, error) {
	var names []string
	err := fs.WalkDir(Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(p), ".md") {
			return err
		}
		names = append(names, "docs/"+p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = fs.WalkDir(extcatalog.Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(p), ".md") {
			return err
		}
		names = append(names, extcatalog.PathPrefix+p)
		return nil
	})
	if dipNames, err := dips.MarkdownNames(); err == nil {
		names = append(names, dipNames...)
	}
	return names, nil
}
