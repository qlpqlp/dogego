// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

// DefaultMaxTipAge is Core DEFAULT_MAX_TIP_AGE (-maxtipage): tip older than this stays in IBD.
const DefaultMaxTipAge = 86400

// EffectiveMaxTipAge returns seconds for IsInitialBlockDownload-style checks (0 or negative → default).
func EffectiveMaxTipAge(sec int) int64 {
	if sec <= 0 {
		return DefaultMaxTipAge
	}
	return int64(sec)
}
