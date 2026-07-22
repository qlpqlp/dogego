// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"net/http"

	"dogego/version"
)

func versionFields() map[string]any {
	return map[string]any{
		"client_version":    version.Display(),
		"dogego_version":    version.Display(),
		"dogego_beta":       version.Beta,
		"dogego_user_agent": version.HTTPUserAgent(),
		"client_brand":      version.ClientName,
	}
}

func mergeVersionFields(m map[string]any) {
	for k, v := range versionFields() {
		m[k] = v
	}
}

func userAgentMiddleware(next http.Handler) http.Handler {
	ua := version.HTTPUserAgent()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("User-Agent", ua)
		w.Header().Set("X-DogeGo-Version", version.Display())
		if version.Beta {
			w.Header().Set("X-DogeGo-Beta", "1")
		}
		next.ServeHTTP(w, r)
	})
}
