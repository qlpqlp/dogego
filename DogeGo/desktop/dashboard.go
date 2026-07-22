// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"strings"

	"dogego/config"
)

// DefaultURLScheme is the custom protocol name (dogecoin://node, dogecoin:ADDRESS).
const DefaultURLScheme = "dogecoin"

// DashboardURL builds the browser URL for the local web UI from saved config.
func DashboardURL(f config.File) string {
	if f.NoWebUI {
		return ""
	}
	scheme := "http"
	if f.WebUIHTTPS() {
		scheme = "https"
	}
	webui := strings.TrimSpace(f.WebUI)
	if webui == "" {
		return scheme + "://" + config.DefaultWebUIListen + "/"
	}
	if strings.HasPrefix(webui, "http://") || strings.HasPrefix(webui, "https://") {
		if strings.HasSuffix(webui, "/") {
			return webui
		}
		return webui + "/"
	}
	return scheme + "://" + webui + "/"
}
