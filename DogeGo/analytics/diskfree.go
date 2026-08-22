// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

// VolumeUsage reports free and total bytes for the filesystem that holds path.
// Returns free=0, total=0, err!=nil when the probe is unavailable.
func VolumeUsage(path string) (free, total uint64, err error) {
	return volumeUsage(path)
}
