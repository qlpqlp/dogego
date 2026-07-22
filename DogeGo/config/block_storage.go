// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"strings"

	"dogego/store"
)

// ApplyRecommendedStorageDefaults sets bundled blk*.dat, zstd compression, and compact tx
// index (offset-only) for new full-node installs. No-op for SPV.
func ApplyRecommendedStorageDefaults(f *File) {
	if f == nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(f.NodeMode)) == "spv" {
		return
	}
	f.BlockStorageLayout = store.BlockLayoutBundled
	f.BlockZstd = true
	embedOff := false
	f.TxIndexEmbedTx = &embedOff
}

// SetupWizardSeed returns wizard defaults merged onto any CLI/file seed.
func SetupWizardSeed(base File) File {
	out := base
	ApplyRecommendedStorageDefaults(&out)
	ApplyRecommendedNetworkDefaults(&out)
	ApplyRecommendedSecurityDefaults(&out)
	if strings.TrimSpace(out.DataDir) == "" {
		out.DataDir = "dogedata"
	}
	if abs, err := ResolveDataDir(out.DataDir); err == nil && abs != "" {
		out.DataDir = abs
	}
	return out
}

// EffectiveBlockStorageOpts maps dogecoinconf.json fields to store layout options.
func (f File) EffectiveBlockStorageOpts() store.BlockStorageOpts {
	return store.NormalizeBlockStorageOpts(store.BlockStorageOpts{
		Layout: f.BlockStorageLayout,
		Zstd:   f.BlockZstd,
	})
}

// EffectiveTxIndexEmbedTx reports whether indexes/tx should embed serialized tx bytes (v2).
func (f File) EffectiveTxIndexEmbedTx() bool {
	if f.TxIndexEmbedTx != nil && !*f.TxIndexEmbedTx {
		return false
	}
	return true
}

// EffectiveBlockStorageOpts for merged runtime config.
func (m Merged) EffectiveBlockStorageOpts() store.BlockStorageOpts {
	return store.NormalizeBlockStorageOpts(store.BlockStorageOpts{
		Layout: m.BlockStorageLayout,
		Zstd:   m.BlockZstd,
	})
}

// EffectiveTxIndexEmbedTx for merged runtime config.
func (m Merged) EffectiveTxIndexEmbedTx() bool {
	if !m.TxIndexEmbedTx {
		return false
	}
	return true
}
