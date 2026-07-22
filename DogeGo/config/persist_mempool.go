// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

// EffectivePersistMempool reports whether dogego_mempool.json is auto-loaded/saved (default true).
func EffectivePersistMempool(f File) bool {
	if f.PersistMempool != nil {
		return *f.PersistMempool
	}
	return true
}
