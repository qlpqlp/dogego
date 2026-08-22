// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import "strings"

// NotifyUpdateAvailable shows a native desktop notification when a newer release is found.
func NotifyUpdateAvailable(latestVersion, releaseURL string) {
	latestVersion = strings.TrimSpace(latestVersion)
	if latestVersion == "" {
		return
	}
	title := "DogeGo update available"
	body := "Version " + latestVersion + " is on GitHub Releases."
	if u := strings.TrimSpace(releaseURL); u != "" {
		body += " Open the dashboard or tray menu to install."
	}
	platformNotify(title, body)
}

// NotifyDiskSpaceLow shows a native desktop notification when the datadir volume is nearly full.
func NotifyDiskSpaceLow(message, advice string) {
	title := "DogeGo: low disk space"
	body := strings.TrimSpace(message)
	if body == "" {
		body = "The datadir drive is nearly full. Full block download is paused."
	}
	if a := strings.TrimSpace(advice); a != "" {
		body += " " + a
	}
	platformNotify(title, body)
}
