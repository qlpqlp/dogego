// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dips

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed *.md
var Files embed.FS

// PathPrefix is the stable path prefix used by the Docs markdown viewer.
const PathPrefix = "DIPs/"

// ReadMarkdown returns DIP markdown for DIPs/NAME.md or NAME.md.
func ReadMarkdown(rel string) ([]byte, string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(rel, "/")
	rel = strings.TrimPrefix(rel, PathPrefix)
	rel = strings.TrimPrefix(rel, "dips/")
	rel = path.Clean(rel)
	if rel == "." || rel == ".." || strings.Contains(rel, "..") {
		return nil, "", fmt.Errorf("invalid path")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".md") {
		return nil, "", fmt.Errorf("not a markdown file")
	}
	b, err := Files.ReadFile(rel)
	if err != nil {
		return nil, "", err
	}
	return b, PathPrefix + rel, nil
}

// IsDIPMarkdownPath reports whether rel refers to a DIP markdown file.
func IsDIPMarkdownPath(rel string) bool {
	rel = strings.TrimSpace(strings.ToLower(rel))
	rel = strings.TrimPrefix(rel, "/")
	return strings.HasPrefix(rel, "dips/") || strings.HasPrefix(rel, strings.ToLower(PathPrefix))
}

// MarkdownNames lists embedded DIP markdown paths.
func MarkdownNames() ([]string, error) {
	var names []string
	err := fs.WalkDir(Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(p), ".md") {
			return err
		}
		if strings.EqualFold(p, "README.md") {
			names = append(names, PathPrefix+"README.md")
			return nil
		}
		names = append(names, PathPrefix+p)
		return nil
	})
	return names, err
}
