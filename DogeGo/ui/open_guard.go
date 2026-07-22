// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

func openDebouncePath(url string) string {
	key := openURLKey(url)
	sum := sha256.Sum256([]byte(key))
	name := ".dogego-open-" + hex.EncodeToString(sum[:8]) + ".lock"
	return filepath.Join(os.TempDir(), name)
}

func openDebounceActive(url string) bool {
	path := openDebouncePath(url)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < openURLDebounce
}

func markOpenDebounceFile(url string) {
	path := openDebouncePath(url)
	_ = os.WriteFile(path, []byte(time.Now().Format(time.RFC3339Nano)), 0o600)
	go func() {
		time.Sleep(openURLDebounce)
		_ = os.Remove(path)
	}()
}

// WasOpenedRecently reports whether this URL was opened recently (in-process or cross-process).
func WasOpenedRecently(url string) bool {
	if openDebounceActive(url) {
		return true
	}
	key := openURLKey(url)
	openURLMu.Lock()
	defer openURLMu.Unlock()
	return key == lastOpenURL && time.Since(lastOpenURLAt) < openURLDebounce
}
