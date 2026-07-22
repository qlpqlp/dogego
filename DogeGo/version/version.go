// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package version is the single source of truth for DogeGo release metadata (like Core clientversion).
package version

import (
	"fmt"
	"strings"
	"unicode"
)

// ClientName is the P2P sub-version client segment (/Name:Version/).
const ClientName = "DogeGo"

// ClientVersion is bumped on each DogeGo release (semver).
const ClientVersion = "0.1.0"

// CoreBaseVersion is the Dogecoin Core release this implementation targets (consensus/P2P parity baseline).
const CoreBaseVersion = "1.14.9"

// Beta marks pre-release builds (shown in UI, CLI, and user-agent when true).
const Beta = true

// Display returns the human-facing version string (e.g. "0.1.0-beta (1.14.9)").
func Display() string {
	s := ClientVersion
	if Beta {
		s += "-beta"
	}
	return s + " (" + CoreBaseVersion + ")"
}

// Banner returns the startup line printed to the console.
func Banner() string {
	return fmt.Sprintf("DogeGo %s - Much Faster Full Dogecoin Node", Display())
}

// HTTPUserAgent is sent on dashboard HTTP responses.
func HTTPUserAgent() string {
	return ClientName + "/" + Display()
}

// BuildSubVersion returns the P2P wire sub-version, e.g. /DogeGo:0.1.0-beta (1.14.9)/ or /DogeGo:0.1.0-beta (1.14.9)(note)/.
func BuildSubVersion(uaComment string) string {
	c := sanitizeUAComment(uaComment)
	out := "/" + ClientName + ":" + Display()
	if c != "" {
		out += "(" + c + ")"
	}
	return out + "/"
}

func sanitizeUAComment(uaComment string) string {
	c := strings.TrimSpace(uaComment)
	var b strings.Builder
	for _, r := range c {
		if r == '/' || r == '(' || r == ')' {
			continue
		}
		if !unicode.IsPrint(r) && r != '\t' {
			continue
		}
		b.WriteRune(r)
	}
	c = strings.TrimSpace(b.String())
	if len(c) > 200 {
		c = c[:200]
	}
	return c
}
