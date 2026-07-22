// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package version

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LatestDownloadedAsset returns the newest verified update binary under baseDataDir/updates/ for latestVersion.
func LatestDownloadedAsset(baseDataDir, latestVersion string) (string, error) {
	dir := filepath.Join(baseDataDir, "updates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	prefix := "dogego-" + strings.TrimSpace(latestVersion)
	var best string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), ".sha256") {
			continue
		}
		best = filepath.Join(dir, name)
	}
	if best == "" {
		return "", fmt.Errorf("no downloaded update for %s in %s", latestVersion, dir)
	}
	return best, nil
}
