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

const (
	ManifestFileName   = "dogego.extension.json"
	ManifestVersion    = 1
	MaxManifestBytes   = 256 * 1024
	MaxExtensionIDLen  = 64
)

var extensionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9-]*){1,8}$`)
var rpcMethodPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// EntryType describes how an extension module is loaded.
type EntryType string

const (
	EntryBuiltin    EntryType = "builtin"
	EntrySubprocess EntryType = "subprocess"
	EntryWasm       EntryType = "wasm"
)

// Manifest is the dogego.extension.json v1 schema.
type Manifest struct {
	ManifestVersion int      `json:"manifest_version"`
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Author          string   `json:"author,omitempty"`
	Description     string   `json:"description,omitempty"`
	Homepage        string   `json:"homepage,omitempty"`
	Repository      string   `json:"repository,omitempty"`
	DogeGoMinVersion string  `json:"dogego_min_version,omitempty"`
	Permissions     []string `json:"permissions,omitempty"`
	Networks        []string `json:"networks,omitempty"`
	Entry           Entry    `json:"entry"`
	Capabilities    []string `json:"capabilities,omitempty"`
	UI              ManifestUI `json:"ui,omitempty"`
	RPC             []RPCMethod `json:"rpc,omitempty"`
	Icon            string   `json:"icon,omitempty"`
	DocsPath        string   `json:"docs_path,omitempty"`
}

// Entry points at the loadable module.
type Entry struct {
	Type     EntryType         `json:"type"`
	Module   string            `json:"module"`
	Binary   string            `json:"binary,omitempty"`
	Binaries map[string]string `json:"binaries,omitempty"` // platform key → path inside zip (see BUILDING.md)
	Wasm     string            `json:"wasm,omitempty"`
}

var allowedPermissions = map[string]struct{}{
	"chain_read":     {},
	"chain_index":    {},
	"datadir_write":  {},
	"p2p_extension":  {},
	"rpc_register":   {},
	"ui_panel":       {},
	"wallet_rpc":     {},
}

var forbiddenPermissions = map[string]struct{}{
	"wallet":         {},
	"private_keys":   {},
	"sign_message":   {},
	"sign_transaction": {},
	"spend":          {},
}

func stripUTF8BOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

// LoadManifest reads and validates dogego.extension.json from dir.
func LoadManifest(dir string) (Manifest, error) {
	path := filepath.Join(dir, ManifestFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	if len(raw) > MaxManifestBytes {
		return Manifest{}, fmt.Errorf("extension manifest too large")
	}
	raw = stripUTF8BOM(raw)
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("extension manifest json: %w", err)
	}
	if err := ValidateManifest(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// ValidateManifest checks manifest fields and security policy.
func ValidateManifest(m Manifest) error {
	if m.ManifestVersion != ManifestVersion {
		return fmt.Errorf("unsupported manifest_version %d (want %d)", m.ManifestVersion, ManifestVersion)
	}
	id := strings.TrimSpace(m.ID)
	if id == "" || len(id) > MaxExtensionIDLen {
		return fmt.Errorf("invalid extension id")
	}
	if !extensionIDPattern.MatchString(id) {
		return fmt.Errorf("extension id %q must match reverse-dns form (e.g. dogego.zkl2)", id)
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("extension name required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("extension version required")
	}
	if err := validateEntry(m.Entry); err != nil {
		return err
	}
	for _, p := range m.Permissions {
		p = strings.TrimSpace(strings.ToLower(p))
		if _, bad := forbiddenPermissions[p]; bad {
			return fmt.Errorf("forbidden permission %q", p)
		}
		if _, ok := allowedPermissions[p]; !ok {
			return fmt.Errorf("unknown permission %q", p)
		}
	}
	for _, p := range m.Permissions {
		if _, bad := forbiddenPermissions[strings.ToLower(strings.TrimSpace(p))]; bad {
			return fmt.Errorf("forbidden permission %q", p)
		}
	}
	for _, rm := range m.RPC {
		if err := validateRPCMethodName(rm.Name); err != nil {
			return err
		}
	}
	if err := ValidateIconRel(m.Icon); err != nil {
		return err
	}
	return nil
}

func validateRPCMethodName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("rpc method name required")
	}
	if strings.HasPrefix(name, "dogego_") {
		return fmt.Errorf("rpc method %q reserved (dogego_ prefix)", name)
	}
	if !rpcMethodPattern.MatchString(name) {
		return fmt.Errorf("rpc method %q must match [a-z][a-z0-9_]{0,31}", name)
	}
	return nil
}

func validateEntry(e Entry) error {
	switch e.Type {
	case EntryBuiltin, EntrySubprocess, EntryWasm:
	default:
		return fmt.Errorf("unsupported entry type %q", e.Type)
	}
	if strings.TrimSpace(e.Module) == "" && e.Type == EntryBuiltin {
		return fmt.Errorf("entry.module required for builtin extensions")
	}
	if e.Type == EntrySubprocess && strings.TrimSpace(e.Binary) == "" {
		return fmt.Errorf("entry.binary required for subprocess extensions")
	}
	if e.Type == EntrySubprocess {
		for k, v := range e.Binaries {
			if err := validatePlatformKey(k); err != nil {
				return err
			}
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("entry.binaries[%q] path required", k)
			}
			if strings.Contains(v, "..") {
				return fmt.Errorf("entry.binaries[%q] invalid path", k)
			}
		}
	}
	if e.Type == EntryWasm && strings.TrimSpace(e.Wasm) == "" {
		return fmt.Errorf("entry.wasm required for wasm extensions")
	}
	return nil
}

// SupportsNetwork reports whether the extension may run on network name.
func (m Manifest) SupportsNetwork(network string) bool {
	network = strings.ToLower(strings.TrimSpace(network))
	if len(m.Networks) == 0 {
		return true
	}
	for _, n := range m.Networks {
		if strings.EqualFold(strings.TrimSpace(n), network) {
			return true
		}
	}
	return false
}

// HasPermission checks a declared permission.
func (m Manifest) HasPermission(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, p := range m.Permissions {
		if strings.EqualFold(strings.TrimSpace(p), name) {
			return true
		}
	}
	return false
}
