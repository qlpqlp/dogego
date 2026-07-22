// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dogego/version"
)

// DefaultCatalogURL is the official DogeGo extensions catalog (GitHub raw JSON).
const DefaultCatalogURL = "https://raw.githubusercontent.com/qlpqlp/dogego/main/DogeGo/extensions/catalog/catalog.json"

const (
	catalogCacheFile = "catalog.cache.json"
	catalogTTL       = 6 * time.Hour
	catalogMaxBytes  = 2 << 20
	catalogHTTPTimeout = 12 * time.Second
)

// CatalogEntry is one extension in the remote catalog.
type CatalogEntry struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Version          string   `json:"version"`
	Author           string   `json:"author,omitempty"`
	Description      string   `json:"description,omitempty"`
	Homepage         string   `json:"homepage,omitempty"`
	Repository       string   `json:"repository,omitempty"`
	DownloadURL      string                        `json:"download_url,omitempty"`
	SHA256           string                        `json:"sha256,omitempty"`
	Downloads        map[string]PlatformArtifact   `json:"downloads,omitempty"`
	DogeGoMinVersion string   `json:"dogego_min_version,omitempty"`
	Networks         []string `json:"networks,omitempty"`
	Builtin          bool     `json:"builtin,omitempty"`
	Featured         bool     `json:"featured,omitempty"`
	Permissions      []string `json:"permissions,omitempty"`
	Capabilities     []string `json:"capabilities,omitempty"`
	Icon             string   `json:"icon,omitempty"`
	DocsPath         string   `json:"docs_path,omitempty"`
}

// CatalogFile is the remote extensions catalog JSON schema.
type CatalogFile struct {
	CatalogVersion int            `json:"catalog_version"`
	Updated        string         `json:"updated,omitempty"`
	Source         string         `json:"source,omitempty"`
	Extensions     []CatalogEntry `json:"extensions"`
}

// CatalogRow merges remote catalog metadata with local install state.
type CatalogRow struct {
	CatalogEntry
	Installed        bool     `json:"installed"`
	Enabled          bool     `json:"enabled"`
	Status           string   `json:"status,omitempty"`
	Builtin          bool     `json:"builtin"`
	RPCMethods       []string `json:"rpc_methods,omitempty"`
	UIPanel          bool     `json:"ui_panel,omitempty"`
	UIStatusMethod   string   `json:"ui_status_method,omitempty"`
	InstalledVersion string   `json:"installed_version,omitempty"`
	UpdateAvailable  bool     `json:"update_available,omitempty"`
}

type catalogCache struct {
	FetchedAtUnix int64       `json:"fetched_at_unix"`
	Sources       []string    `json:"sources,omitempty"`
	URL           string      `json:"url,omitempty"` // legacy single-source cache
	Catalog       CatalogFile `json:"catalog"`
}

func catalogSourcesKey(sources []string) string {
	return strings.Join(sources, "\n")
}

// SetCatalogURL overrides the default catalog fetch URL (empty restores default).
func (m *Manager) SetCatalogURL(url string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.catalogURL = strings.TrimSpace(url)
	if m.catalogURL == "" {
		m.catalogURL = DefaultCatalogURL
	}
}

func (m *Manager) catalogURLLocked() string {
	if m.catalogURL == "" {
		return DefaultCatalogURL
	}
	return m.catalogURL
}

// FetchCatalog downloads (or loads cache) the merged extensions catalog.
func (m *Manager) FetchCatalog(ctx context.Context, force bool) (CatalogFile, error) {
	if m == nil {
		return CatalogFile{}, fmt.Errorf("extensions unwired")
	}
	m.mu.Lock()
	m.loadCatalogSourcesLocked()
	sources := append([]string(nil), m.catalogSources...)
	cachePath := filepath.Join(m.rootDir, catalogCacheFile)
	m.mu.Unlock()

	if !force {
		if cat, ok := m.loadCatalogCache(cachePath, sources); ok {
			return cat, nil
		}
	}
	cat, err := m.fetchMergedCatalog(ctx, sources)
	if err != nil {
		if stale, ok := m.loadCatalogCacheStale(cachePath); ok {
			return stale, nil
		}
		return CatalogFile{}, err
	}
	_ = m.saveCatalogCache(cachePath, sources, cat)
	return cat, nil
}

func (m *Manager) fetchMergedCatalog(ctx context.Context, sources []string) (CatalogFile, error) {
	if len(sources) == 0 {
		sources = defaultCatalogSources()
	}
	seen := make(map[string]struct{})
	var merged CatalogFile
	merged.CatalogVersion = 1
	var lastErr error
	for _, url := range sources {
		raw, err := m.httpGetCatalog(ctx, url)
		if err != nil {
			lastErr = err
			continue
		}
		var part CatalogFile
		if err := json.Unmarshal(raw, &part); err != nil {
			lastErr = err
			continue
		}
		if part.CatalogVersion < 1 {
			lastErr = fmt.Errorf("unsupported catalog_version in %s", url)
			continue
		}
		if merged.Source == "" && part.Source != "" {
			merged.Source = part.Source
		}
		if merged.Updated == "" && part.Updated != "" {
			merged.Updated = part.Updated
		}
		for _, e := range part.Extensions {
			if _, ok := seen[e.ID]; ok {
				continue
			}
			seen[e.ID] = struct{}{}
			merged.Extensions = append(merged.Extensions, e)
		}
	}
	if len(merged.Extensions) == 0 && lastErr != nil {
		return CatalogFile{}, lastErr
	}
	return merged, nil
}

// ListCatalog merges remote catalog entries with installed/builtin state.
func (m *Manager) ListCatalog(ctx context.Context, forceRefresh bool) ([]CatalogRow, error) {
	cat, err := m.FetchCatalog(ctx, forceRefresh)
	if err != nil {
		// Offline or catalog fetch failed: still list builtins + locally installed zips.
		cat = CatalogFile{CatalogVersion: 1, Extensions: nil}
	}
	local := m.List()
	byID := make(map[string]InstalledRow, len(local))
	for _, r := range local {
		byID[r.ID] = r
	}
	seen := make(map[string]struct{})
	var out []CatalogRow
	for _, e := range cat.Extensions {
		e.DocsPath = EnrichDocsPath(e.ID, e.DocsPath)
		row := CatalogRow{CatalogEntry: e}
		row.UIPanel = catalogEntryUIPanel(e)
		if lr, ok := byID[e.ID]; ok {
			row.Installed = lr.Installed
			row.Enabled = lr.Enabled
			row.Status = lr.Status
			row.Builtin = lr.Builtin
			row.RPCMethods = append([]string(nil), lr.RPCMethods...)
			if lr.UIPanel {
				row.UIPanel = true
			}
			row.UIStatusMethod = lr.UIStatusMethod
			row.InstalledVersion = strings.TrimSpace(lr.Version)
			if row.Installed && row.InstalledVersion != "" && strings.TrimSpace(e.Version) != "" {
				row.UpdateAvailable = version.SemverCompare(e.Version, row.InstalledVersion) > 0
			}
			if len(lr.Permissions) > 0 {
				row.Permissions = append([]string(nil), lr.Permissions...)
			}
			if len(lr.Capabilities) > 0 {
				row.Capabilities = append([]string(nil), lr.Capabilities...)
			}
			if row.Icon == "" && lr.Icon != "" {
				row.Icon = lr.Icon
			}
			if row.DocsPath == "" && lr.DocsPath != "" {
				row.DocsPath = lr.DocsPath
			}
		}
		if row.Builtin || e.Builtin {
			row.Builtin = true
		}
		out = append(out, row)
		seen[e.ID] = struct{}{}
	}
	for _, lr := range local {
		if _, ok := seen[lr.ID]; ok {
			continue
		}
		if !lr.Installed {
			continue
		}
		out = append(out, CatalogRow{
			CatalogEntry: CatalogEntry{
				ID:           lr.ID,
				Name:         lr.Name,
				Version:      lr.Version,
				Author:       lr.Author,
				Description:  lr.Description,
				Homepage:     lr.Homepage,
				Repository:   lr.Repository,
				Builtin:      lr.Builtin,
				Permissions:  append([]string(nil), lr.Permissions...),
				Capabilities: append([]string(nil), lr.Capabilities...),
				Icon:         lr.Icon,
				DocsPath:     EnrichDocsPath(lr.ID, lr.DocsPath),
			},
			Installed:      lr.Installed,
			Enabled:        lr.Enabled,
			Status:         lr.Status,
			Builtin:        lr.Builtin,
			RPCMethods:     append([]string(nil), lr.RPCMethods...),
			UIPanel:        lr.UIPanel,
			UIStatusMethod: lr.UIStatusMethod,
		})
	}
	return out, nil
}

func catalogEntryUIPanel(e CatalogEntry) bool {
	for _, p := range e.Permissions {
		if strings.EqualFold(strings.TrimSpace(p), "ui_panel") {
			return true
		}
	}
	return false
}

func catalogCompatible(entry CatalogEntry, network string) bool {
	if len(entry.Networks) == 0 {
		return true
	}
	net := strings.ToLower(strings.TrimSpace(network))
	for _, n := range entry.Networks {
		if strings.EqualFold(strings.TrimSpace(n), net) {
			return true
		}
	}
	return false
}

func catalogVersionOK(entry CatalogEntry) error {
	minV := strings.TrimSpace(entry.DogeGoMinVersion)
	if minV == "" {
		return nil
	}
	if version.SemverCompare(version.ClientVersion, minV) < 0 {
		return fmt.Errorf("extension %q requires DogeGo >= %s (running %s)", entry.ID, minV, version.ClientVersion)
	}
	return nil
}

func (m *Manager) httpGetCatalog(ctx context.Context, url string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: catalogHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("catalog HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, catalogMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > catalogMaxBytes {
		return nil, fmt.Errorf("catalog exceeds size limit")
	}
	return raw, nil
}

func (m *Manager) loadCatalogCache(path string, sources []string) (CatalogFile, bool) {
	var cc catalogCache
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &cc) != nil {
		return CatalogFile{}, false
	}
	key := catalogSourcesKey(sources)
	cached := catalogSourcesKey(cc.Sources)
	if cached == "" && cc.URL != "" {
		cached = cc.URL
	}
	if cached != key {
		return CatalogFile{}, false
	}
	if cc.FetchedAtUnix <= 0 {
		return CatalogFile{}, false
	}
	if time.Since(time.Unix(cc.FetchedAtUnix, 0)) > catalogTTL {
		return CatalogFile{}, false
	}
	return cc.Catalog, true
}

func (m *Manager) loadCatalogCacheStale(path string) (CatalogFile, bool) {
	var cc catalogCache
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &cc) != nil {
		return CatalogFile{}, false
	}
	return cc.Catalog, cc.Catalog.CatalogVersion > 0
}

func (m *Manager) saveCatalogCache(path string, sources []string, cat CatalogFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	cc := catalogCache{
		FetchedAtUnix: time.Now().Unix(),
		Sources:       append([]string(nil), sources...),
		Catalog:       cat,
	}
	raw, err := json.MarshalIndent(cc, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
