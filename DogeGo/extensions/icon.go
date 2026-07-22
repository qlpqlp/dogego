// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dogego/extensions/catalog"
)

const defaultExtensionIcon = "icon.png"

// BuiltinCatalogDir maps built-in extension ids to catalog package folders.
var BuiltinCatalogDir = map[string]string{
	"dogego.zkl2":     "zkl2",
	"dogego.doginals": "doginals",
	"example.go":      "example-go",
	"example.wasm":    "example-wasm",
}

// NormalizeIconRel returns a safe relative PNG path inside an extension package.
func NormalizeIconRel(icon string) string {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return defaultExtensionIcon
	}
	icon = strings.TrimPrefix(icon, "/")
	icon = strings.ReplaceAll(icon, "\\", "/")
	icon = filepath.ToSlash(filepath.Clean(icon))
	if icon == "." || icon == ".." || strings.HasPrefix(icon, "../") {
		return defaultExtensionIcon
	}
	if !strings.HasSuffix(strings.ToLower(icon), ".png") {
		return defaultExtensionIcon
	}
	return icon
}

// ValidateIconRel rejects unsafe icon paths in manifests.
func ValidateIconRel(icon string) error {
	icon = strings.TrimSpace(icon)
	if icon == "" {
		return nil
	}
	if strings.HasPrefix(icon, "/") || strings.Contains(icon, "..") {
		return fmt.Errorf("invalid icon path %q", icon)
	}
	rel := NormalizeIconRel(icon)
	if strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") {
		return fmt.Errorf("invalid icon path %q", icon)
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".png") {
		return fmt.Errorf("extension icon must be a .png file (got %q)", icon)
	}
	return nil
}

// ResolveIconBytes loads a PNG for id using mgr when wired, then catalog builtins.
func ResolveIconBytes(m *Manager, id string) ([]byte, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("extension id required")
	}
	if m != nil {
		if b, err := m.IconBytes(id); err == nil && len(b) > 0 {
			return b, nil
		}
	}
	if dir, ok := BuiltinCatalogDir[id]; ok {
		if b, err := catalog.ReadIconBytes(dir, defaultExtensionIcon); err == nil && len(b) > 0 {
			return b, nil
		}
	}
	return nil, fmt.Errorf("extension icon not found for %q", id)
}

// IconBytes returns PNG bytes for extension id (installed dir, then catalog embed/disk).
func (m *Manager) IconBytes(id string) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("extensions unwired")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("extension id required")
	}
	m.mu.Lock()
	root := m.rootDir
	m.mu.Unlock()

	rel := defaultExtensionIcon
	installDir := filepath.Join(root, id)
	if man, err := LoadManifest(installDir); err == nil {
		rel = NormalizeIconRel(man.Icon)
		if b, err := os.ReadFile(filepath.Join(installDir, filepath.FromSlash(rel))); err == nil {
			return b, nil
		}
	}
	if dir, ok := BuiltinCatalogDir[id]; ok {
		if b, err := catalog.ReadIconBytes(dir, rel); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("extension icon not found for %q", id)
}
