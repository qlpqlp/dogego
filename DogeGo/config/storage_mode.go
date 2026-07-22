// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"fmt"
	"strings"
)

const StorageNative = "native"

// ParseStorageMode rejects legacy Core storage modes; only native layout is supported.
func ParseStorageMode(s string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "", StorageNative:
		return StorageNative, nil
	case "core", "core_readonly", "dual":
		return "", fmt.Errorf("storage_mode %q removed: DogeGo uses native layout only (headers.bin, rawblocks/, indexes/)", strings.TrimSpace(s))
	default:
		return "", fmt.Errorf("storage_mode: want native (got %q)", s)
	}
}
