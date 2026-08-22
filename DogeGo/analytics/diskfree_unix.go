// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

//go:build unix

package analytics

import (
	"fmt"
	"path/filepath"
	"syscall"
)

func volumeUsage(path string) (free, total uint64, err error) {
	path = filepath.Clean(path)
	if path == "" {
		return 0, 0, fmt.Errorf("empty path")
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	bsize := uint64(st.Bsize)
	if bsize == 0 {
		bsize = 1
	}
	total = uint64(st.Blocks) * bsize
	free = uint64(st.Bavail) * bsize
	return free, total, nil
}
