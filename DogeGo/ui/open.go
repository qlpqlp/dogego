// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const openURLDebounce = 1500 * time.Millisecond

var (
	openURLMu     sync.Mutex
	lastOpenURL   string
	lastOpenURLAt time.Time
)

func openURLKey(url string) string {
	url = strings.TrimSpace(url)
	if url == "" {
		return "(default)"
	}
	return url
}

func markOpenURL(url string) {
	openURLMu.Lock()
	lastOpenURL = openURLKey(url)
	lastOpenURLAt = time.Now()
	openURLMu.Unlock()
}

// OpenURL opens the system default browser (Windows, macOS, Linux).
func OpenURL(url string) error {
	return openURLPlatform(url)
}

// OpenURLLog opens the URL and prints a hint on failure (non-fatal).
// Rapid duplicate opens for the same URL are ignored (systray / protocol handler storms).
func OpenURLLog(url string) {
	openURLLog(url, false)
}

// OpenURLForce opens the URL even if the same address was opened recently.
// Use for explicit user actions (tray "Open Dashboard") so debounce never swallows a click.
func OpenURLForce(url string) {
	openURLLog(url, true)
}

func openURLLog(url string, force bool) {
	if strings.TrimSpace(url) == "" {
		return
	}
	if !force && WasOpenedRecently(url) {
		return
	}
	markOpenDebounceFile(url)
	markOpenURL(url)
	// Never block callers (systray menu loop, HTTP handlers) on ShellExecute / xdg-open.
	go func(u string) {
		if err := OpenURL(u); err != nil {
			fmt.Fprintf(os.Stderr, "DogeGo web UI: could not open browser (%v). Open manually: %s\n", err, u)
		}
	}(url)
}
