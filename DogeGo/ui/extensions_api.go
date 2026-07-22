// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"dogego/ui/websecurity"
	"dogego/extensions"
)

const extZipUploadMax = extensions.MaxExtensionZipBytes

func registerExtensionsRoutes(mux *http.ServeMux, cfg StartConfig, webGate *websecurity.Gate) {
	if mux == nil || cfg.RPCInvoke == nil {
		return
	}
	invoke := cfg.RPCInvoke
	mux.HandleFunc("/api/extensions", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		res := invoke("dogego_listextensions", nil)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/api/extensions/catalog", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		force := r.URL.Query().Get("refresh") == "1"
		raw, _ := json.Marshal(force)
		res := invoke("dogego_listextensioncatalog", []json.RawMessage{raw})
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/api/extensions/enable", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		raw, _ := json.Marshal(body.ID)
		res := invoke("dogego_enableextension", []json.RawMessage{raw})
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/api/extensions/disable", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		raw, _ := json.Marshal(body.ID)
		res := invoke("dogego_disableextension", []json.RawMessage{raw})
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/api/extensions/install", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "multipart/form-data") {
			if err := r.ParseMultipartForm(extZipUploadMax); err != nil {
				http.Error(w, "bad upload", http.StatusBadRequest)
				return
			}
			f, hdr, err := r.FormFile("zip")
			if err != nil {
				http.Error(w, "missing zip file", http.StatusBadRequest)
				return
			}
			defer f.Close()
			tmp, err := os.CreateTemp("", "dogego-ext-*.zip")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tmpPath := tmp.Name()
			n, copyErr := io.Copy(tmp, io.LimitReader(f, extZipUploadMax+1))
			_ = tmp.Close()
			if copyErr != nil || n > extZipUploadMax {
				os.Remove(tmpPath)
				http.Error(w, "zip too large", http.StatusBadRequest)
				return
			}
			_ = hdr
			raw, _ := json.Marshal(tmpPath)
			res := invoke("dogego_instextensionzip", []json.RawMessage{raw})
			os.Remove(tmpPath)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(res)
			return
		}
		var body struct {
			ID  string `json:"id"`
			URL string `json:"url"`
			SHA string `json:"sha256,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var res interface{}
		if body.ID != "" {
			raw, _ := json.Marshal(body.ID)
			res = invoke("dogego_instextension", []json.RawMessage{raw})
		} else if body.URL != "" {
			low := strings.ToLower(strings.TrimSpace(body.URL))
			if !strings.HasPrefix(low, "https://") {
				http.Error(w, "only https urls allowed", http.StatusBadRequest)
				return
			}
			rawURL, _ := json.Marshal(body.URL)
			if body.SHA != "" {
				rawSHA, _ := json.Marshal(body.SHA)
				res = invoke("dogego_instextensionurl", []json.RawMessage{rawURL, rawSHA})
			} else {
				res = invoke("dogego_instextensionurl", []json.RawMessage{rawURL})
			}
		} else {
			http.Error(w, "id or url required", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/api/extensions/update", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		// Force catalog refresh then reinstall from GitHub / catalog download_url.
		rawForce, _ := json.Marshal(true)
		_ = invoke("dogego_listextensioncatalog", []json.RawMessage{rawForce})
		raw, _ := json.Marshal(strings.TrimSpace(body.ID))
		res := invoke("dogego_instextension", []json.RawMessage{raw})
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/api/extensions/uninstall", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ID         string `json:"id"`
			RemoveData bool   `json:"remove_data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		rawID, _ := json.Marshal(body.ID)
		rawRm, _ := json.Marshal(body.RemoveData)
		res := invoke("dogego_uninstextension", []json.RawMessage{rawID, rawRm})
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/api/extensions/icon", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			http.NotFound(w, r)
			return
		}
		mgr := cfg.Extensions
		if mgr == nil && cfg.ExtensionManager != nil {
			mgr = cfg.ExtensionManager()
		}
		b, err := extensions.ResolveIconBytes(mgr, id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("/api/extensions/docs", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		docsPath := strings.TrimSpace(r.URL.Query().Get("path"))
		if id != "" {
			listRes := invoke("dogego_listextensioncatalog", nil)
			docsPath = resolveExtensionDocsPath(listRes, id, docsPath)
		}
		docsPath = extensions.EnrichDocsPath(id, docsPath)
		if docsPath == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "no documentation for this extension"})
			return
		}
		b, name, err := ReadEmbeddedMarkdown(docsPath)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "path": docsPath})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":       id,
			"path":     name,
			"markdown": string(b),
		})
	})
	mux.HandleFunc("/api/extensions/panel", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		listRes := invoke("dogego_listextensions", nil)
		method := resolveExtensionPanelRPC(listRes, id)
		if method == "" {
			http.Error(w, "extension panel not available", http.StatusNotFound)
			return
		}
		res := invoke(method, nil)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(res)
	})
	mux.HandleFunc("/api/extensions/catalog-sources", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			res := invoke("dogego_listextensioncatalogsources", nil)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(res)
		case http.MethodPost:
			var body struct {
				URL string `json:"url"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.URL) == "" {
				http.Error(w, "url required", http.StatusBadRequest)
				return
			}
			raw, _ := json.Marshal(strings.TrimSpace(body.URL))
			res := invoke("dogego_addextensioncatalogsource", []json.RawMessage{raw})
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(res)
		case http.MethodDelete:
			u := strings.TrimSpace(r.URL.Query().Get("url"))
			if u == "" {
				http.Error(w, "url required", http.StatusBadRequest)
				return
			}
			raw, _ := json.Marshal(u)
			res := invoke("dogego_removeextensioncatalogsource", []json.RawMessage{raw})
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(res)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// resolveExtensionPanelRPC finds the status RPC for an enabled ui_panel extension.
func resolveExtensionPanelRPC(listRes interface{}, id string) string {
	rows := extensions.ParseListRPCResult(listRes)
	for _, row := range rows {
		if row.ID != id {
			continue
		}
		if !row.UIPanel || !row.Enabled {
			return ""
		}
		inner := strings.TrimSpace(row.UIStatusMethod)
		if inner == "" {
			inner = extensions.DefaultUIStatusMethod
		}
		return extensions.PanelStatusRPC(id, inner)
	}
	return ""
}

func resolveExtensionDocsPath(catalogRes interface{}, id, fallback string) string {
	if p := strings.TrimSpace(fallback); p != "" {
		return p
	}
	raw, err := json.Marshal(catalogRes)
	if err != nil {
		return extensions.EnrichDocsPath(id, "")
	}
	var nested struct {
		Result struct {
			Catalog []extensions.CatalogRow `json:"catalog"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &nested); err == nil {
		for _, row := range nested.Result.Catalog {
			if row.ID == id && strings.TrimSpace(row.DocsPath) != "" {
				return row.DocsPath
			}
		}
	}
	return extensions.EnrichDocsPath(id, "")
}
