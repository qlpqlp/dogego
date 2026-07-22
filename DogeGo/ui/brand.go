// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"io/fs"
	"net/http"
	"strings"
)

// addBrandingRoutes serves the official Dogecoin mark (same SVG as share/pixmaps/dogecoin256.svg in Core).
func addBrandingRoutes(mux *http.ServeMux) {
	serveStaticFile(mux, "/app.css", "static/app.css", "text/css; charset=utf-8")
	serveStaticFile(mux, "/app.js", "static/app.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/security.js", "static/security.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/ui-prefs.js", "static/ui-prefs.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/ui-controls.js", "static/ui-controls.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/i18n.js", "static/i18n.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/num-format.js", "static/num-format.js", "application/javascript; charset=utf-8")
	for _, lang := range []string{"en", "fr", "pt-PT", "de", "zh", "ja"} {
		serveStaticFile(mux, "/locales/"+lang+".json", "static/locales/"+lang+".json", "application/json; charset=utf-8")
	}
	serveStaticFile(mux, "/blockstep.js", "static/blockstep.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/doge-wait.js", "static/doge-wait.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/sync-dock.js", "static/sync-dock.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/tx-flight.js", "static/tx-flight.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/wallet_passphrase.js", "static/wallet_passphrase.js", "application/javascript; charset=utf-8")
	serveStaticFile(mux, "/qrcode.min.js", "static/qrcode.min.js", "application/javascript; charset=utf-8")
	svgData, err := fs.ReadFile(static, "static/dogecoin.svg")
	if err != nil {
		return
	}
	mux.HandleFunc("/dogecoin.svg", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(svgData)
	})
	serveStaticFile(mux, "/dogecoin-foundation.svg", "static/dogecoin-foundation.svg", "image/svg+xml; charset=utf-8")
	serveStaticFile(mux, "/dogecoin_testnet.svg", "static/dogecoin_testnet.svg", "image/svg+xml; charset=utf-8")
	// Tab icon (Core uses .ico; we ship the same artwork as SVG for broad browser support).
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dogecoin.svg", http.StatusTemporaryRedirect)
	})
}

func serveStaticFile(mux *http.ServeMux, route, path, contentType string) {
	data, err := fs.ReadFile(static, path)
	if err != nil {
		return
	}
	mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		if strings.HasSuffix(path, ".svg") {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		_, _ = w.Write(data)
	})
}
