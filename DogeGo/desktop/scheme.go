// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RegisterURLScheme registers the dogecoin:// handler for the current user.
func RegisterURLScheme(scheme string) error {
	scheme = strings.TrimSpace(scheme)
	if scheme == "" {
		scheme = DefaultURLScheme
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	return registerURLSchemePlatform(scheme, exe)
}

// UnregisterURLScheme removes the per-user dogecoin:// handler.
func UnregisterURLScheme(scheme string) error {
	scheme = strings.TrimSpace(scheme)
	if scheme == "" {
		scheme = DefaultURLScheme
	}
	return unregisterURLSchemePlatform(scheme)
}

// URLSchemeStatus reports whether the custom protocol is registered for this user.
func URLSchemeStatus(scheme string) (registered bool, detail string, err error) {
	scheme = strings.TrimSpace(scheme)
	if scheme == "" {
		scheme = DefaultURLScheme
	}
	return urlSchemeStatusPlatform(scheme)
}

// HandlerCommand returns the shell command registered for a scheme (for display).
func HandlerCommand(exePath string) string {
	exePath = strings.TrimSpace(exePath)
	if exePath == "" {
		return fmt.Sprintf("%s open --url \"%%1\"", "dogego")
	}
	if strings.ContainsAny(exePath, " \t") {
		exePath = `"` + exePath + `"`
	}
	return exePath + ` open --url "%1"`
}
