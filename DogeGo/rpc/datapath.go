// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateFilePath resolves filename under allowed roots. When allowExternal is false,
// paths outside roots are rejected (probe/dump). Import flows pass allowExternal true.
func ValidateFilePath(roots []string, filename string, allowExternal bool) (string, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "", fmt.Errorf("invalid filename")
	}
	clean := filepath.Clean(filename)
	if clean == "." || clean == string(os.PathSeparator) {
		return "", fmt.Errorf("invalid filename")
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("invalid filename")
	}
	if allowExternal {
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("file not found")
		}
		return abs, nil
	}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if pathWithinRoot(rootAbs, abs) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("path must be under the node datadir")
}

func pathWithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(target, root+sep)
}

func dataPathRoots(paths *DataPaths) []string {
	if paths == nil {
		return nil
	}
	var roots []string
	if paths.BaseDataDir != "" {
		roots = append(roots, paths.BaseDataDir)
	}
	if paths.ChainDataDir != "" && paths.ChainDataDir != paths.BaseDataDir {
		roots = append(roots, paths.ChainDataDir)
	}
	return roots
}

// PathsWithDataDir returns DataPaths with BaseDataDir and ChainDataDir set to dir.
func PathsWithDataDir(dir string) *DataPaths {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return &DataPaths{BaseDataDir: abs, ChainDataDir: abs}
}

// mergePathsDataDir fills empty datadir fields on paths from dir.
func mergePathsDataDir(paths *DataPaths, dir string) *DataPaths {
	if paths == nil {
		return PathsWithDataDir(dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	if paths.BaseDataDir == "" {
		paths.BaseDataDir = abs
	}
	if paths.ChainDataDir == "" {
		paths.ChainDataDir = abs
	}
	return paths
}
