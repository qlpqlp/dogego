// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package node

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const uncleanShutdownMarker = ".dogego-unclean-shutdown"

// MarkUncleanShutdown records that a shutdown started. Cleared only after a clean flush.
// If the process is force-killed or force-exits, the next start runs repair.
func MarkUncleanShutdown(chainDir string) {
	chainDir = strings.TrimSpace(chainDir)
	if chainDir == "" {
		return
	}
	path := filepath.Join(chainDir, uncleanShutdownMarker)
	body := []byte("shutdown_started=" + time.Now().UTC().Format(time.RFC3339) + "\n")
	_ = os.WriteFile(path, body, 0o600)
}

// ClearUncleanShutdown removes the unclean-shutdown marker after a successful clean exit flush.
func ClearUncleanShutdown(chainDir string) {
	chainDir = strings.TrimSpace(chainDir)
	if chainDir == "" {
		return
	}
	_ = os.Remove(filepath.Join(chainDir, uncleanShutdownMarker))
}

// HasUncleanShutdown reports whether the previous run did not finish a clean shutdown flush.
func HasUncleanShutdown(chainDir string) bool {
	chainDir = strings.TrimSpace(chainDir)
	if chainDir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(chainDir, uncleanShutdownMarker))
	return err == nil
}
