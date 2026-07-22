// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseGitHubTreeURL splits a GitHub tree URL into owner, repo, branch, and subpath.
func ParseGitHubTreeURL(u string) (owner, repo, branch, subpath string, ok bool) {
	m := githubTreePattern.FindStringSubmatch(strings.TrimSpace(u))
	if len(m) != 5 {
		return "", "", "", "", false
	}
	return m[1], m[2], m[3], strings.TrimSuffix(m[4], "/"), true
}

// InstallFromGitHubTree downloads a GitHub folder and installs the extension (builds Go subprocess if needed).
func (m *Manager) InstallFromGitHubTree(ctx context.Context, treeURL string) (InstalledRow, error) {
	if m == nil {
		return InstalledRow{}, fmt.Errorf("extensions unwired")
	}
	owner, repo, branch, subpath, ok := ParseGitHubTreeURL(treeURL)
	if !ok {
		return InstalledRow{}, fmt.Errorf("not a github tree url: %s", treeURL)
	}
	archiveURL := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/refs/heads/%s", owner, repo, branch)
	tmpZip, err := m.downloadToTemp(ctx, archiveURL)
	if err != nil {
		return InstalledRow{}, fmt.Errorf("github archive: %w", err)
	}
	defer os.Remove(tmpZip)
	prefix := fmt.Sprintf("%s-%s/", repo, branch)
	if subpath != "" {
		prefix = prefix + subpath + "/"
	}
	return m.installFromArchiveSubdir(tmpZip, prefix)
}

func (m *Manager) installFromArchiveSubdir(zipPath, entryPrefix string) (InstalledRow, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return InstalledRow{}, err
	}
	defer r.Close()
	if err := os.MkdirAll(m.rootDir, 0o755); err != nil {
		return InstalledRow{}, err
	}
	tmpDir, err := os.MkdirTemp(m.rootDir, "github-install-*")
	if err != nil {
		return InstalledRow{}, err
	}
	defer os.RemoveAll(tmpDir)

	var written int64
	found := false
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, entryPrefix) {
			continue
		}
		found = true
		rel := strings.TrimPrefix(f.Name, entryPrefix)
		if rel == "" || strings.HasPrefix(rel, "/") {
			continue
		}
		clean := filepath.Clean(rel)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return InstalledRow{}, fmt.Errorf("zip path traversal: %q", f.Name)
		}
		dest := filepath.Join(tmpDir, clean)
		if !strings.HasPrefix(dest, tmpDir+string(os.PathSeparator)) && dest != tmpDir {
			return InstalledRow{}, fmt.Errorf("zip path escape: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return InstalledRow{}, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return InstalledRow{}, err
		}
		n, err := extractZipFile(f, dest)
		written += n
		if written > maxZipExtractBytes {
			return InstalledRow{}, fmt.Errorf("extension archive exceeds size limit")
		}
		if err != nil {
			return InstalledRow{}, err
		}
	}
	if !found {
		return InstalledRow{}, fmt.Errorf("github folder not found in archive (prefix %q)", entryPrefix)
	}
	man, err := LoadManifest(tmpDir)
	if err != nil {
		return InstalledRow{}, err
	}
	destDir := filepath.Join(m.rootDir, man.ID)
	if err := os.RemoveAll(destDir); err != nil {
		return InstalledRow{}, err
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		return InstalledRow{}, err
	}
	if err := materializePlatformBinary(destDir, man); err != nil {
		os.RemoveAll(destDir)
		return InstalledRow{}, err
	}
	if err := buildSubprocessIfNeeded(destDir, man); err != nil {
		os.RemoveAll(destDir)
		return InstalledRow{}, err
	}
	row := manifestRow(man, man.Entry.Type == EntryBuiltin, true)
	return row, nil
}

// InstallFromRepository installs from a catalog repository field (GitHub tree URL).
func (m *Manager) InstallFromRepository(ctx context.Context, repository string) (InstalledRow, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return InstalledRow{}, fmt.Errorf("empty repository url")
	}
	if owner, _, _, _, ok := ParseGitHubTreeURL(repository); ok && owner != "" {
		return m.InstallFromGitHubTree(ctx, repository)
	}
	return InstalledRow{}, fmt.Errorf("unsupported repository url %q (use a GitHub tree link)", repository)
}
