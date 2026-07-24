// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package catalog

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

const manifestFileName = "dogego.extension.json"

// PackageMeta is one catalog package discovered from dogego.extension.json.
type PackageMeta struct {
	ID       string
	Dir      string // folder under extensions/catalog/
	Icon     string
	DocsPath string
}

var (
	packagesOnce sync.Once
	packagesByID map[string]PackageMeta
)

// Packages returns a copy of id → package metadata for every discovered package.
func Packages() map[string]PackageMeta {
	ensurePackages()
	out := make(map[string]PackageMeta, len(packagesByID))
	for k, v := range packagesByID {
		out[k] = v
	}
	return out
}

// PackageDir returns the catalog folder for an extension id.
func PackageDir(id string) (string, bool) {
	ensurePackages()
	p, ok := packagesByID[strings.TrimSpace(id)]
	if !ok || p.Dir == "" {
		return "", false
	}
	return p.Dir, true
}

// PackageDocsPath returns docs_path from the package manifest, if any.
func PackageDocsPath(id string) string {
	ensurePackages()
	p, ok := packagesByID[strings.TrimSpace(id)]
	if !ok {
		return ""
	}
	return strings.TrimSpace(p.DocsPath)
}

// PackageIconRel returns the icon relative path from the package manifest (default icon.png).
func PackageIconRel(id string) string {
	ensurePackages()
	p, ok := packagesByID[strings.TrimSpace(id)]
	if !ok {
		return ""
	}
	icon := strings.TrimSpace(p.Icon)
	if icon == "" {
		return "icon.png"
	}
	return icon
}

func ensurePackages() {
	packagesOnce.Do(func() {
		packagesByID = discoverPackages()
	})
}

func discoverPackages() map[string]PackageMeta {
	out := make(map[string]PackageMeta)
	scanManifestTree(Files, ".", out)
	if root, ok := moduleRootDir(); ok {
		diskRoot := filepath.Join(root, "extensions", "catalog")
		entries, err := os.ReadDir(diskRoot)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				dir := e.Name()
				raw, err := os.ReadFile(filepath.Join(diskRoot, dir, manifestFileName))
				if err != nil {
					continue
				}
				if meta, ok := parsePackageMeta(dir, raw); ok {
					if _, exists := out[meta.ID]; !exists {
						out[meta.ID] = meta
					}
				}
			}
		}
	}
	return out
}

func scanManifestTree(fsys fs.FS, root string, out map[string]PackageMeta) {
	_ = fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		p = path.Clean(strings.ReplaceAll(p, "\\", "/"))
		if path.Base(p) != manifestFileName {
			return nil
		}
		dir := path.Dir(p)
		if dir == "." || strings.Contains(dir, "/") {
			return nil // only <pkg>/dogego.extension.json
		}
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil
		}
		if meta, ok := parsePackageMeta(dir, raw); ok {
			out[meta.ID] = meta
		}
		return nil
	})
}

func parsePackageMeta(dir string, raw []byte) (PackageMeta, bool) {
	var man struct {
		ID       string `json:"id"`
		Icon     string `json:"icon"`
		DocsPath string `json:"docs_path"`
	}
	if json.Unmarshal(raw, &man) != nil {
		return PackageMeta{}, false
	}
	id := strings.TrimSpace(man.ID)
	if id == "" || strings.TrimSpace(dir) == "" {
		return PackageMeta{}, false
	}
	return PackageMeta{
		ID:       id,
		Dir:      strings.TrimSpace(dir),
		Icon:     strings.TrimSpace(man.Icon),
		DocsPath: strings.TrimSpace(man.DocsPath),
	}, true
}
