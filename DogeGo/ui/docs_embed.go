// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"dogego/docs"
)

// ReadEmbeddedMarkdown returns repo markdown bundled with the DogeGo build.
func ReadEmbeddedMarkdown(rel string) ([]byte, string, error) {
	return docs.ReadMarkdown(rel)
}

// EmbeddedMarkdownFiles returns doc paths for the Docs tab file picker.
func EmbeddedMarkdownFiles() []string {
	n, err := docs.MarkdownNames()
	if err != nil || len(n) == 0 {
		return nil
	}
	return n
}
