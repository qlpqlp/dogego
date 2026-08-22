// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

//go:build !windows && !unix

package analytics

import "fmt"

func volumeUsage(path string) (free, total uint64, err error) {
	return 0, 0, fmt.Errorf("volume usage not supported on this platform")
}
