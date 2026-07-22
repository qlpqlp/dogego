// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"fmt"
	"strings"

	"dogego/config"
	"dogego/ui"
)

// ResolveOpenURL maps dogecoin://, http(s)://, or empty input to a dashboard URL.
func ResolveOpenURL(raw string, f config.File) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		u := DashboardURL(f)
		if u == "" {
			return "", fmt.Errorf("web UI is disabled (nowebui)")
		}
		return u, nil
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, DefaultURLScheme+":") {
		return resolveCustomScheme(raw, f)
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if strings.HasSuffix(raw, "/") {
			return raw, nil
		}
		return raw + "/", nil
	}
	return "", fmt.Errorf("unsupported URL %q (use %s://node, %s:ADDRESS, or http://localhost:2013/)", raw, DefaultURLScheme, DefaultURLScheme)
}

// OpenDashboard opens the configured dashboard in the default browser.
func OpenDashboard(f config.File) error {
	u, err := ResolveOpenURL("", f)
	if err != nil {
		return err
	}
	ui.OpenURLForce(u)
	return nil
}

// OpenURL opens a resolved dashboard or custom-scheme URL in the default browser.
func OpenURL(raw string, f config.File) error {
	u, err := ResolveOpenURL(raw, f)
	if err != nil {
		return err
	}
	OpenURLLog(u)
	return nil
}

// OpenURLLog opens a URL string in the default browser (non-fatal logging on failure).
func OpenURLLog(url string) {
	ui.OpenURLLog(url)
}

// OpenURLForce opens a URL from an explicit user action (tray click); never debounce-drops.
func OpenURLForce(url string) {
	ui.OpenURLForce(url)
}
