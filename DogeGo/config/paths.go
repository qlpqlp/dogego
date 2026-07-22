// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "path/filepath"

// ResolveDataDir turns relative datadir paths into absolute paths (stable across cwd changes).
func ResolveDataDir(dir string) (string, error) {
	dir = filepath.Clean(dir)
	if dir == "" || filepath.IsAbs(dir) {
		return dir, nil
	}
	return filepath.Abs(dir)
}
