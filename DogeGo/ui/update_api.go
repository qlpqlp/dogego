// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"

	"dogego/version"
)

func registerUpdateRoutes(mux *http.ServeMux, cfg StartConfig) {
	mux.HandleFunc("/api/update/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeUpdateJSON(w, cfg)
	})
	mux.HandleFunc("/api/update/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if cfg.UpdateChecker == nil {
			http.Error(w, "update checker not available", http.StatusServiceUnavailable)
			return
		}
		cfg.UpdateChecker.RefreshNow(r.Context())
		writeUpdateJSON(w, cfg)
	})
	mux.HandleFunc("/api/update/dismiss", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if cfg.UpdateChecker == nil {
			http.Error(w, "update checker not available", http.StatusServiceUnavailable)
			return
		}
		if err := cfg.UpdateChecker.Dismiss(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/update/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if cfg.UpdateChecker == nil {
			http.Error(w, "update checker not available", http.StatusServiceUnavailable)
			return
		}
		st := cfg.UpdateChecker.Status()
		if st.DownloadURL == "" {
			http.Error(w, "no direct download asset for this platform", http.StatusBadRequest)
			return
		}
		res, err := cfg.UpdateChecker.DownloadReleaseAssetVerified(cfg.BaseDataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		note := "Stop DogeGo, replace your binary with the downloaded file, then restart."
		if res.ChecksumVerify {
			note = "SHA256 verified. Use Install update to restart into the new binary automatically, or replace manually."
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":              true,
			"path":            res.Path,
			"sha256":          res.SHA256,
			"checksum_verify": res.ChecksumVerify,
			"note":            note,
			"build_cmd":       st.BuildCommand,
			"release_url":     st.ReleaseURL,
			"latest":          st.LatestVersion,
		})
	})
	mux.HandleFunc("/api/update/apply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if cfg.UpdateChecker == nil {
			http.Error(w, "update checker not available", http.StatusServiceUnavailable)
			return
		}
		if cfg.ApplyUpdate == nil {
			http.Error(w, "apply update not available", http.StatusServiceUnavailable)
			return
		}
		st := cfg.UpdateChecker.Status()
		if !st.Available || st.DownloadURL == "" {
			http.Error(w, "no update available for this platform", http.StatusBadRequest)
			return
		}
		path, err := version.LatestDownloadedAsset(cfg.BaseDataDir, st.LatestVersion)
		if err != nil {
			res, dlErr := cfg.UpdateChecker.DownloadReleaseAssetVerified(cfg.BaseDataDir)
			if dlErr != nil {
				http.Error(w, dlErr.Error(), http.StatusInternalServerError)
				return
			}
			path = res.Path
			if st.ChecksumURL != "" && !res.ChecksumVerify {
				http.Error(w, "release checksum required before apply", http.StatusBadRequest)
				return
			}
		} else if st.ChecksumURL != "" || st.ChecksumSHA256 != "" {
			if _, err := cfg.UpdateChecker.VerifyDownloadedAsset(path); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if err := cfg.ApplyUpdate(path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"path": path,
			"note": "Update process starting. The node will restart shortly.",
		})
	})
}

func writeUpdateJSON(w http.ResponseWriter, cfg StartConfig) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if cfg.UpdateChecker == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "note": "update checker not wired"})
		return
	}
	out := cfg.UpdateChecker.SummaryFields()
	out["ok"] = true
	_ = json.NewEncoder(w).Encode(out)
}

func mergeUpdateFields(m map[string]any, cfg StartConfig) {
	if cfg.UpdateChecker == nil {
		return
	}
	for k, v := range cfg.UpdateChecker.SummaryFields() {
		m[k] = v
	}
}
