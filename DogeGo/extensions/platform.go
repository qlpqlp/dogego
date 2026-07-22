// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

var platformKeyPattern = regexp.MustCompile(`^[a-z0-9]+-[a-z0-9]+$`)

// PlatformArtifact is a per-platform catalog download (zip URL + optional sha256).
type PlatformArtifact struct {
	DownloadURL string `json:"download_url"`
	SHA256      string `json:"sha256,omitempty"`
}

// CurrentPlatformKey returns the host platform id (goos-goarch), e.g. windows-amd64.
func CurrentPlatformKey() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// SelectPlatformArtifact picks a catalog download for the running host.
// Tries exact platform key, then "universal" (fat zip with entry.binaries inside).
func SelectPlatformArtifact(downloads map[string]PlatformArtifact) (key string, art PlatformArtifact, err error) {
	if len(downloads) == 0 {
		return "", PlatformArtifact{}, fmt.Errorf("empty downloads map")
	}
	want := CurrentPlatformKey()
	if a, ok := downloads[want]; ok && strings.TrimSpace(a.DownloadURL) != "" {
		return want, a, nil
	}
	if a, ok := downloads["universal"]; ok && strings.TrimSpace(a.DownloadURL) != "" {
		return "universal", a, nil
	}
	var keys []string
	for k := range downloads {
		keys = append(keys, k)
	}
	return "", PlatformArtifact{}, fmt.Errorf("no download for platform %q (catalog has: %s)", want, strings.Join(keys, ", "))
}

// SelectPlatformBinaryPath picks a relative path inside an installed extension dir.
func SelectPlatformBinaryPath(binaries map[string]string) (platformKey, relPath string, err error) {
	if len(binaries) == 0 {
		return "", "", fmt.Errorf("empty binaries map")
	}
	want := CurrentPlatformKey()
	if rel, ok := binaries[want]; ok && strings.TrimSpace(rel) != "" {
		return want, strings.TrimSpace(rel), nil
	}
	if rel, ok := binaries["universal"]; ok && strings.TrimSpace(rel) != "" {
		return "universal", strings.TrimSpace(rel), nil
	}
	var keys []string
	for k := range binaries {
		keys = append(keys, k)
	}
	return "", "", fmt.Errorf("no binary for platform %q (manifest has: %s)", want, strings.Join(keys, ", "))
}

func validatePlatformKey(key string) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "universal" {
		return nil
	}
	if !platformKeyPattern.MatchString(key) {
		return fmt.Errorf("invalid platform key %q (want goos-goarch, e.g. windows-amd64, or universal)", key)
	}
	return nil
}
