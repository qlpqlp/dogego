// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/json"
	"os"
)

// SegmentManifestSnapshot is the on-disk headers/manifest.json fields used by dashboard/RPC.
type SegmentManifestSnapshot struct {
	TipHeight  int64
	TipHashHex string
}

func readSegmentManifestFile(chainDir string) (SegmentManifestSnapshot, bool) {
	if chainDir == "" {
		return SegmentManifestSnapshot{}, false
	}
	b, err := os.ReadFile(headerManifestPath(chainDir))
	if err != nil {
		return SegmentManifestSnapshot{}, false
	}
	var m headerManifest
	if err := json.Unmarshal(b, &m); err != nil || m.TipHeight < 0 {
		return SegmentManifestSnapshot{}, false
	}
	return SegmentManifestSnapshot{TipHeight: m.TipHeight, TipHashHex: m.TipHashHex}, true
}

// ReadSegmentManifestTip reads headers/manifest.json tip height (0 if missing/empty).
func ReadSegmentManifestTip(chainDir string) (tip int64, ok bool) {
	m, ok := readSegmentManifestFile(chainDir)
	if !ok {
		return 0, false
	}
	return m.TipHeight, true
}

// ReadSegmentManifest reads tip height and hash from headers/manifest.json without journal locks.
func ReadSegmentManifest(chainDir string) (SegmentManifestSnapshot, bool) {
	return readSegmentManifestFile(chainDir)
}
