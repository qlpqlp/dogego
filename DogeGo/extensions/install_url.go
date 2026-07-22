// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	installHTTPTimeout = 120 * time.Second
	installMaxBytes    = MaxExtensionZipBytes
)

// InstallFromURL downloads a verified zip and installs it.
func (m *Manager) InstallFromURL(ctx context.Context, url, wantSHA256 string) (InstalledRow, error) {
	if m == nil {
		return InstalledRow{}, fmt.Errorf("extensions unwired")
	}
	url = strings.TrimSpace(url)
	if url == "" {
		return InstalledRow{}, fmt.Errorf("empty download url")
	}
	if !strings.HasPrefix(strings.ToLower(url), "https://") {
		return InstalledRow{}, fmt.Errorf("only https download urls are allowed")
	}
	if _, _, _, _, ok := ParseGitHubTreeURL(url); ok {
		return m.InstallFromGitHubTree(ctx, url)
	}
	tmp, err := m.downloadToTemp(ctx, url)
	if err != nil {
		return InstalledRow{}, err
	}
	defer os.Remove(tmp)
	if sha := strings.TrimSpace(strings.ToLower(wantSHA256)); sha != "" {
		if err := verifyFileSHA256(tmp, sha); err != nil {
			return InstalledRow{}, err
		}
	}
	return m.InstallZip(tmp)
}

// InstallCatalogEntry installs a catalog row (builtin entries are no-ops).
func (m *Manager) InstallCatalogEntry(ctx context.Context, id string) (InstalledRow, error) {
	cat, err := m.FetchCatalog(ctx, false)
	if err != nil {
		return InstalledRow{}, err
	}
	var entry *CatalogEntry
	for i := range cat.Extensions {
		if cat.Extensions[i].ID == id {
			entry = &cat.Extensions[i]
			break
		}
	}
	if entry == nil {
		return InstalledRow{}, fmt.Errorf("extension %q not in catalog", id)
	}
	if entry.Builtin && entry.Repository == "" && entry.DownloadURL == "" && len(entry.Downloads) == 0 {
		row := manifestRow(DefaultBuiltinManifest(entry.ID), true, m.isInstalledLocked(entry.ID))
		return row, nil
	}
	if !catalogCompatible(*entry, m.network) {
		return InstalledRow{}, fmt.Errorf("extension %q not supported on network %q", id, m.network)
	}
	if err := catalogVersionOK(*entry); err != nil {
		return InstalledRow{}, err
	}
	if len(entry.Downloads) > 0 {
		plat, art, err := SelectPlatformArtifact(entry.Downloads)
		if err != nil {
			return InstalledRow{}, err
		}
		row, err := m.InstallFromURL(ctx, art.DownloadURL, art.SHA256)
		if err != nil {
			return InstalledRow{}, fmt.Errorf("install %s (%s): %w", id, plat, err)
		}
		return row, nil
	}
	if entry.DownloadURL != "" {
		return m.InstallFromURL(ctx, entry.DownloadURL, entry.SHA256)
	}
	if strings.TrimSpace(entry.Repository) != "" {
		return m.InstallFromRepository(ctx, entry.Repository)
	}
	return InstalledRow{}, fmt.Errorf("catalog entry %q has no downloads, download_url, or repository", id)
}

func DefaultBuiltinManifest(id string) Manifest {
	return Manifest{ID: id, Name: id, Version: "0.0.0", Entry: Entry{Type: EntryBuiltin, Module: id}}
}

func (m *Manager) downloadToTemp(ctx context.Context, url string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: installHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(m.rootDir, 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(m.rootDir, "dl-*.zip")
	if err != nil {
		return "", err
	}
	path := f.Name()
	n, err := io.Copy(f, io.LimitReader(resp.Body, installMaxBytes+1))
	_ = f.Close()
	if err != nil {
		os.Remove(path)
		return "", err
	}
	if n > installMaxBytes {
		os.Remove(path)
		return "", fmt.Errorf("download exceeds size limit")
	}
	return path, nil
}

func verifyFileSHA256(path, want string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, strings.TrimSpace(want)) {
		return fmt.Errorf("sha256 mismatch (got %s)", got)
	}
	return nil
}

// Uninstall removes a non-builtin extension from disk.
func (m *Manager) Uninstall(id string, removeData bool) error {
	if m == nil {
		return fmt.Errorf("extensions unwired")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("empty extension id")
	}

	m.mu.Lock()
	ext := m.active[id]
	delete(m.active, id)
	delete(m.activeManifest, id)
	delete(m.enabled, id)
	root := m.rootDir
	_ = m.saveStateLocked()
	m.mu.Unlock()

	if ext != nil {
		_ = ext.OnDisable()
	}

	dir := filepath.Join(root, id)
	man, _ := LoadManifest(dir)
	forceKillExtensionBinary(dir, man.Entry.Binary)
	killStaleSubprocess(filepath.Join(dir, "data"))

	if removeData {
		return removePathRetry(dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, ent := range entries {
		if ent.Name() == "data" {
			continue
		}
		if err := removePathRetry(filepath.Join(dir, ent.Name())); err != nil {
			return err
		}
	}
	return nil
}
