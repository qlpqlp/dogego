// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxZipExtractBytes = MaxExtensionZipExtractBytes

// InstallZip extracts an extension archive into extensions/<id>/.
// An existing extension data/ directory (databases, settings) is preserved across updates.
func (m *Manager) InstallZip(zipPath string) (InstalledRow, error) {
	if m == nil {
		return InstalledRow{}, fmt.Errorf("extensions unwired")
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return InstalledRow{}, err
	}
	defer r.Close()

	_, prefix, err := locateManifestInZip(r.File)
	if err != nil {
		return InstalledRow{}, err
	}
	if err := os.MkdirAll(m.rootDir, 0o755); err != nil {
		return InstalledRow{}, err
	}
	tmpDir, err := os.MkdirTemp(m.rootDir, "install-*")
	if err != nil {
		return InstalledRow{}, err
	}
	defer os.RemoveAll(tmpDir)

	var written int64
	for _, f := range r.File {
		name := strings.TrimPrefix(f.Name, prefix)
		if name == "" || strings.HasPrefix(name, "/") {
			continue
		}
		clean := filepath.Clean(name)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return InstalledRow{}, fmt.Errorf("zip path traversal: %q", f.Name)
		}
		// Never extract a zip "data/" tree over the operator's live extension database.
		if clean == "data" || strings.HasPrefix(clean, "data"+string(os.PathSeparator)) || strings.HasPrefix(clean, "data/") {
			continue
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
			return InstalledRow{}, fmt.Errorf("extension zip exceeds size limit")
		}
		if err != nil {
			return InstalledRow{}, err
		}
	}

	man, err := LoadManifest(tmpDir)
	if err != nil {
		return InstalledRow{}, err
	}
	destDir := filepath.Join(m.rootDir, man.ID)

	// Preserve databases and settings under extensions/<id>/data/.
	dataDir := filepath.Join(destDir, "data")
	var preservedData string
	defer func() {
		if preservedData != "" {
			_ = os.RemoveAll(preservedData)
		}
	}()
	if st, err := os.Stat(dataDir); err == nil && st.IsDir() {
		preservedData, err = os.MkdirTemp(m.rootDir, "preserve-data-*")
		if err != nil {
			return InstalledRow{}, err
		}
		if err := renameDirContents(dataDir, preservedData); err != nil {
			return InstalledRow{}, fmt.Errorf("preserve extension data: %w", err)
		}
	}

	wasEnabled := false
	m.mu.Lock()
	if ext := m.active[man.ID]; ext != nil {
		wasEnabled = true
		delete(m.active, man.ID)
		delete(m.activeManifest, man.ID)
		m.mu.Unlock()
		_ = ext.OnDisable()
	} else {
		m.mu.Unlock()
	}
	forceKillExtensionBinary(destDir, man.Entry.Binary)
	killStaleSubprocess(filepath.Join(destDir, "data"))

	if err := os.RemoveAll(destDir); err != nil {
		return InstalledRow{}, err
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		return InstalledRow{}, err
	}
	if preservedData != "" {
		restore := filepath.Join(destDir, "data")
		_ = os.RemoveAll(restore)
		if err := os.Rename(preservedData, restore); err != nil {
			if err := copyDirRecursive(preservedData, restore); err != nil {
				return InstalledRow{}, fmt.Errorf("restore extension data: %w", err)
			}
		}
		preservedData = "" // defer RemoveAll no-ops if renamed away
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
	if wasEnabled {
		if err := m.Enable(man.ID); err != nil {
			row.Status = "installed"
			return row, fmt.Errorf("updated but re-enable failed: %w", err)
		}
		row.Enabled = true
		row.Status = "enabled"
	}
	return row, nil
}

func renameDirContents(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, ent := range entries {
		from := filepath.Join(src, ent.Name())
		to := filepath.Join(dst, ent.Name())
		if err := os.Rename(from, to); err != nil {
			if err := copyPath(from, to); err != nil {
				return err
			}
			_ = os.RemoveAll(from)
		}
	}
	return nil
}

func copyDirRecursive(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyPath(path, target)
	})
}

func copyPath(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func locateManifestInZip(files []*zip.File) (manifestRel, prefix string, err error) {
	for _, f := range files {
		base := filepath.Base(f.Name)
		if base != ManifestFileName {
			continue
		}
		dir := filepath.Dir(f.Name)
		if dir == "." {
			return f.Name, "", nil
		}
		prefix = dir + "/"
		return f.Name, prefix, nil
	}
	return "", "", fmt.Errorf("zip missing %s", ManifestFileName)
}

func extractZipFile(f *zip.File, dest string) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode().Perm())
	if err != nil {
		return 0, err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(rc, maxZipExtractBytes+1))
	return n, err
}
