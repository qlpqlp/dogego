// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"strings"

	"dogego/version"
)

func writeUpdateAPIError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": msg})
}

func updateRequestForce(r *http.Request) bool {
	if r == nil {
		return false
	}
	q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("force")))
	if q == "1" || q == "true" || q == "yes" {
		return true
	}
	var body struct {
		Force bool `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.Force
}

func registerUpdateRoutes(mux *http.ServeMux, cfg StartConfig) {
	mux.HandleFunc("/api/update/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeUpdateAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeUpdateJSON(w, cfg)
	})
	mux.HandleFunc("/api/update/check", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeUpdateAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !isLoopback(r) {
			writeUpdateAPIError(w, http.StatusForbidden, "forbidden")
			return
		}
		if cfg.UpdateChecker == nil {
			writeUpdateAPIError(w, http.StatusServiceUnavailable, "update checker not available")
			return
		}
		cfg.UpdateChecker.RefreshNow(r.Context())
		writeUpdateJSON(w, cfg)
	})
	mux.HandleFunc("/api/update/dismiss", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeUpdateAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !isLoopback(r) {
			writeUpdateAPIError(w, http.StatusForbidden, "forbidden")
			return
		}
		if cfg.UpdateChecker == nil {
			writeUpdateAPIError(w, http.StatusServiceUnavailable, "update checker not available")
			return
		}
		if err := cfg.UpdateChecker.Dismiss(); err != nil {
			writeUpdateAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/update/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeUpdateAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !isLoopback(r) {
			writeUpdateAPIError(w, http.StatusForbidden, "forbidden")
			return
		}
		if cfg.UpdateChecker == nil {
			writeUpdateAPIError(w, http.StatusServiceUnavailable, "update checker not available")
			return
		}
		st := cfg.UpdateChecker.Status()
		if st.DownloadURL == "" {
			writeUpdateAPIError(w, http.StatusBadRequest, "no direct download asset for this platform (need GOOS+GOARCH match in the release asset name)")
			return
		}
		res, err := cfg.UpdateChecker.DownloadReleaseAssetVerified(cfg.BaseDataDir)
		if err != nil {
			writeUpdateAPIError(w, http.StatusInternalServerError, err.Error())
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
			writeUpdateAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if !isLoopback(r) {
			writeUpdateAPIError(w, http.StatusForbidden, "forbidden")
			return
		}
		if cfg.UpdateChecker == nil {
			writeUpdateAPIError(w, http.StatusServiceUnavailable, "update checker not available")
			return
		}
		if cfg.ApplyUpdate == nil {
			writeUpdateAPIError(w, http.StatusServiceUnavailable, "apply update not available")
			return
		}
		force := updateRequestForce(r)
		st := cfg.UpdateChecker.Status()
		canInstall := st.Installable || (st.DownloadURL != "" && st.LatestVersion != "")
		if !canInstall {
			writeUpdateAPIError(w, http.StatusBadRequest, "no installable release asset for this platform")
			return
		}
		if !force && !st.Available {
			writeUpdateAPIError(w, http.StatusBadRequest, "no newer update available (use Force reinstall to install the GitHub release/pre-release for this platform)")
			return
		}
		path, err := version.LatestDownloadedAsset(cfg.BaseDataDir, st.LatestVersion)
		if err != nil {
			res, dlErr := cfg.UpdateChecker.DownloadReleaseAssetVerified(cfg.BaseDataDir)
			if dlErr != nil {
				writeUpdateAPIError(w, http.StatusInternalServerError, dlErr.Error())
				return
			}
			path = res.Path
			if st.ChecksumURL != "" && !res.ChecksumVerify {
				writeUpdateAPIError(w, http.StatusBadRequest, "release checksum required before apply")
				return
			}
		} else if st.ChecksumURL != "" || st.ChecksumSHA256 != "" {
			if _, err := cfg.UpdateChecker.VerifyDownloadedAsset(path); err != nil {
				writeUpdateAPIError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		if err := cfg.ApplyUpdate(path); err != nil {
			writeUpdateAPIError(w, http.StatusInternalServerError, err.Error())
			return
		}
		note := "Update helper starting: it will stop this process, replace the install binary, and restart DogeGo."
		if force && !st.Available {
			note = "Force reinstall starting for " + st.LatestTag + ". The node will restart shortly."
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    true,
			"path":  path,
			"force": force,
			"note":  note,
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
