// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package docs

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// ResolveMarkdownLink resolves href relative to basePath (e.g. docs/OPERATOR.md or ROADMAP.md).
// fetchPath is suitable for ReadMarkdown. anchor is "#fragment" or empty.
// external is true for http(s) and mailto links.
func ResolveMarkdownLink(basePath, href string) (fetchPath, anchor string, external bool, err error) {
	href = strings.TrimSpace(href)
	if href == "" {
		return "", "", false, fmt.Errorf("empty link")
	}
	if strings.HasPrefix(href, "#") {
		return normalizeDocAPIPath(basePath), href, false, nil
	}
	u, err := url.Parse(href)
	if err != nil {
		return "", "", false, err
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return "", "", true, nil
	}
	if frag := u.Fragment; frag != "" && u.Path == "" && u.Opaque == "" {
		return normalizeDocAPIPath(basePath), "#" + frag, false, nil
	}
	linkPath := u.Path
	if linkPath == "" {
		return "", "", false, fmt.Errorf("unsupported link")
	}
	base := normalizeDocAPIPath(basePath)
	baseDir := path.Dir(base)
	joined := path.Clean(path.Join(baseDir, linkPath))
	if joined == ".." || strings.HasPrefix(joined, "../") || strings.Contains(joined, "/../") {
		return "", "", false, fmt.Errorf("invalid path")
	}
	if !strings.HasSuffix(strings.ToLower(joined), ".md") {
		return "", "", false, fmt.Errorf("not a markdown file")
	}
	anchor = ""
	if u.Fragment != "" {
		anchor = "#" + u.Fragment
	}
	return joined, anchor, false, nil
}

func normalizeDocAPIPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean(p)
	if p == "." {
		return ""
	}
	return p
}
