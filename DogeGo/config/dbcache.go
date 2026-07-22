// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

// Default / bounds for Core-style -dbcache (UTXO working-set budget in MB).
const (
	DefaultDBCacheMB   = 450  // Core-like conservative default when free RAM is unknown
	MinAutoDBCacheMB   = 256
	MaxAutoDBCacheMB   = 16384 // 16 GiB cap even on large hosts
	OSReserveDBCacheMB = 2048  // leave headroom for OS + process overhead
	AutoFreeRAMPercent = 80    // use up to 80% of currently free RAM (after OS reserve)
)

// EffectiveDBCacheMB returns the UTXO/cache budget in megabytes.
// configured > 0 uses that value (capped). configured 0 / omitted = auto from free RAM when
// freeMB >= 0; otherwise DefaultDBCacheMB.
func EffectiveDBCacheMB(configured int, freeMB int64) int {
	if configured > 0 {
		if configured > MaxAutoDBCacheMB {
			return MaxAutoDBCacheMB
		}
		if configured < 64 {
			return 64
		}
		return configured
	}
	if freeMB < 0 {
		return DefaultDBCacheMB
	}
	usable := freeMB - OSReserveDBCacheMB
	if usable < MinAutoDBCacheMB {
		if freeMB >= MinAutoDBCacheMB {
			return MinAutoDBCacheMB
		}
		return DefaultDBCacheMB
	}
	n := int(usable * AutoFreeRAMPercent / 100)
	if n < MinAutoDBCacheMB {
		n = MinAutoDBCacheMB
	}
	if n > MaxAutoDBCacheMB {
		n = MaxAutoDBCacheMB
	}
	return n
}
