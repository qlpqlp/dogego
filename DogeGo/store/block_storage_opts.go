// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Block layout names for raw block payloads under rawblocks/.
const (
	BlockLayoutPerFile = "perfile"
	BlockLayoutBundled = "bundled"
)

// BlockStorageOpts configures on-disk block body layout (Settings / dogecoinconf.json).
// Bundling reduces filesystem overhead (Core-style blk*.dat). Zstd shrinks bytes on disk;
// wire/P2P payloads stay uncompressed when serving peers.
type BlockStorageOpts struct {
	// Layout is "perfile" (default) or "bundled".
	Layout string
	// Zstd compresses stored block bytes (per-file or bundled records).
	Zstd bool
}

// DefaultBlockStorageOpts prefers Core-style bundled blk*.dat (append) over one file per
// block. Per-file IBD on NTFS creates/renames millions of files and cannot beat Core.
func DefaultBlockStorageOpts() BlockStorageOpts {
	return BlockStorageOpts{Layout: BlockLayoutBundled, Zstd: false}
}

// NormalizeBlockStorageOpts trims and validates layout; unknown layout becomes perfile.
func NormalizeBlockStorageOpts(o BlockStorageOpts) BlockStorageOpts {
	o.Layout = strings.ToLower(strings.TrimSpace(o.Layout))
	if o.Layout == "" {
		o.Layout = BlockLayoutPerFile
	}
	if o.Layout != BlockLayoutBundled {
		o.Layout = BlockLayoutPerFile
	}
	return o
}

type blockStorageManifest struct {
	Version int    `json:"version"`
	Layout  string `json:"layout"`
	Zstd    bool   `json:"zstd"`
}

func blockStorageManifestPath(rawDir string) string {
	return filepath.Join(rawDir, "storage.json")
}

// loadBlockStorageManifest reads rawblocks/storage.json when present.
func loadBlockStorageManifest(rawDir string) (BlockStorageOpts, bool) {
	b, err := os.ReadFile(blockStorageManifestPath(rawDir))
	if err != nil {
		return BlockStorageOpts{}, false
	}
	var m blockStorageManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return BlockStorageOpts{}, false
	}
	return NormalizeBlockStorageOpts(BlockStorageOpts{Layout: m.Layout, Zstd: m.Zstd}), true
}

// saveBlockStorageManifest records the active layout after the first stored block.
func saveBlockStorageManifest(rawDir string, opts BlockStorageOpts) error {
	opts = NormalizeBlockStorageOpts(opts)
	b, err := json.Marshal(blockStorageManifest{
		Version: 1,
		Layout:  opts.Layout,
		Zstd:    opts.Zstd,
	})
	if err != nil {
		return err
	}
	tmp := blockStorageManifestPath(rawDir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, blockStorageManifestPath(rawDir))
}

// ResolveBlockStorageOpts merges config opts with an existing on-disk manifest.
// The manifest normally wins so a datadir keeps a stable layout, but config may
// upgrade perfile → bundled, enable zstd, or disable zstd for new Puts (old
// compressed records stay readable via per-record decode).
func ResolveBlockStorageOpts(requested BlockStorageOpts, rawDir string) BlockStorageOpts {
	requested = NormalizeBlockStorageOpts(requested)
	onDisk, ok := loadBlockStorageManifest(rawDir)
	if !ok {
		return requested
	}
	if onDisk.Layout == BlockLayoutPerFile && requested.Layout == BlockLayoutBundled {
		return requested
	}
	if onDisk.Layout == BlockLayoutBundled && requested.Layout == BlockLayoutBundled && requested.Zstd && !onDisk.Zstd {
		return requested
	}
	// Allow operators to turn compression off for IBD without rewriting old bodies.
	if onDisk.Layout == BlockLayoutBundled && requested.Layout == BlockLayoutBundled && onDisk.Zstd && !requested.Zstd {
		return requested
	}
	return onDisk
}

// BlockStorageUpgrade reports whether Resolve would change an existing manifest.
func BlockStorageUpgrade(requested BlockStorageOpts, rawDir string) (from, to BlockStorageOpts, upgrading bool) {
	requested = NormalizeBlockStorageOpts(requested)
	onDisk, ok := loadBlockStorageManifest(rawDir)
	if !ok {
		return BlockStorageOpts{}, requested, false
	}
	resolved := ResolveBlockStorageOpts(requested, rawDir)
	if onDisk.Layout == resolved.Layout && onDisk.Zstd == resolved.Zstd {
		return onDisk, resolved, false
	}
	return onDisk, resolved, true
}
