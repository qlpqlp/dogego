// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultCatalogFolderURL is the human-friendly GitHub catalog folder (normalized to catalog.json).
const DefaultCatalogFolderURL = "https://github.com/qlpqlp/dogego/tree/main/DogeGo/extensions/catalog/"

const catalogSourcesFile = "catalog_sources.json"

var githubTreePattern = regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)/tree/([^/]+)/(.*)$`)

type catalogSourcesConfig struct {
	Sources []string `json:"sources"`
}

func defaultCatalogSources() []string {
	return []string{DefaultCatalogURL, DefaultCatalogFolderURL}
}

func (m *Manager) catalogSourcesPath() string {
	return filepath.Join(m.rootDir, catalogSourcesFile)
}

func (m *Manager) loadCatalogSourcesLocked() {
	if m == nil {
		return
	}
	if len(m.catalogSources) > 0 {
		return
	}
	raw, err := os.ReadFile(m.catalogSourcesPath())
	if err != nil {
		m.catalogSources = defaultCatalogSources()
		return
	}
	var cfg catalogSourcesConfig
	if json.Unmarshal(raw, &cfg) != nil || len(cfg.Sources) == 0 {
		m.catalogSources = defaultCatalogSources()
		return
	}
	m.catalogSources = normalizeCatalogSourceList(cfg.Sources)
}

func (m *Manager) saveCatalogSourcesLocked() error {
	cfg := catalogSourcesConfig{Sources: append([]string(nil), m.catalogSources...)}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.catalogSourcesPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.catalogSourcesPath())
}

func normalizeCatalogSourceList(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, u := range in {
		n, err := NormalizeCatalogSourceURL(u)
		if err != nil {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	if len(out) == 0 {
		return defaultCatalogSources()
	}
	return out
}

// NormalizeCatalogSourceURL maps GitHub folder links to raw catalog.json URLs.
func NormalizeCatalogSourceURL(u string) (string, error) {
	u = strings.TrimSpace(u)
	if u == "" {
		return "", fmt.Errorf("empty catalog url")
	}
	low := strings.ToLower(u)
	if !strings.HasPrefix(low, "https://") {
		return "", fmt.Errorf("only https catalog urls are allowed")
	}
	if m := githubTreePattern.FindStringSubmatch(u); len(m) == 5 {
		owner, repo, branch, sub := m[1], m[2], m[3], strings.TrimSuffix(m[4], "/")
		u = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s/catalog.json", owner, repo, branch, sub)
		return u, nil
	}
	u = strings.TrimSuffix(u, "/")
	if !strings.HasSuffix(strings.ToLower(u), ".json") {
		u += "/catalog.json"
	}
	return u, nil
}

// CatalogSources returns normalized catalog.json fetch URLs.
func (m *Manager) CatalogSources() []string {
	if m == nil {
		return defaultCatalogSources()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadCatalogSourcesLocked()
	return append([]string(nil), m.catalogSources...)
}

// AddCatalogSource appends a catalog source (folder or catalog.json URL).
func (m *Manager) AddCatalogSource(rawURL string) ([]string, error) {
	if m == nil {
		return nil, fmt.Errorf("extensions unwired")
	}
	n, err := NormalizeCatalogSourceURL(rawURL)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadCatalogSourcesLocked()
	for _, existing := range m.catalogSources {
		if existing == n {
			return append([]string(nil), m.catalogSources...), nil
		}
	}
	m.catalogSources = append(m.catalogSources, n)
	if err := m.saveCatalogSourcesLocked(); err != nil {
		return nil, err
	}
	return append([]string(nil), m.catalogSources...), nil
}

// RemoveCatalogSource removes a catalog source by raw or normalized URL.
func (m *Manager) RemoveCatalogSource(rawURL string) ([]string, error) {
	if m == nil {
		return nil, fmt.Errorf("extensions unwired")
	}
	n, err := NormalizeCatalogSourceURL(rawURL)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadCatalogSourcesLocked()
	var next []string
	for _, existing := range m.catalogSources {
		if existing != n {
			next = append(next, existing)
		}
	}
	if len(next) == 0 {
		next = defaultCatalogSources()
	}
	m.catalogSources = next
	if err := m.saveCatalogSourcesLocked(); err != nil {
		return nil, err
	}
	return append([]string(nil), m.catalogSources...), nil
}
