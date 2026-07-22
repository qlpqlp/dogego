// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package docs

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

var rootMarkdownAllow = map[string]struct{}{
	"ROADMAP.md": {},
	"README.md":  {},
}

var (
	moduleRootOnce sync.Once
	moduleRoot     string
	moduleRootOK   bool
)

func readRootMarkdown(name string) ([]byte, error) {
	if _, ok := rootMarkdownAllow[name]; !ok {
		return nil, fs.ErrNotExist
	}
	root, ok := moduleRootDir()
	if !ok {
		return nil, fs.ErrNotExist
	}
	return os.ReadFile(filepath.Join(root, name))
}

func moduleRootDir() (string, bool) {
	moduleRootOnce.Do(func() {
		for _, start := range searchRootDirs() {
			dir := start
			for i := 0; i < 12; i++ {
				if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
					moduleRoot = dir
					moduleRootOK = true
					return
				}
				parent := filepath.Dir(dir)
				if parent == dir {
					break
				}
				dir = parent
			}
		}
	})
	return moduleRoot, moduleRootOK
}

func searchRootDirs() []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(dir string) {
		if dir == "" {
			return
		}
		dir = filepath.Clean(dir)
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
	}
	if wd, err := os.Getwd(); err == nil {
		add(wd)
	}
	if exe, err := os.Executable(); err == nil {
		add(filepath.Dir(exe))
	}
	return out
}
