// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const instancesFileName = "instances.json"

// InstanceEntry describes one DogeGo node process (mainnet + testnet dual-run).
type InstanceEntry struct {
	Network  string `json:"network"`
	WebUI    string `json:"webui"`
	ConfPath string `json:"conf_path"`
	Label    string `json:"label,omitempty"`
}

// InstancesFile lists co-located node processes sharing a base datadir.
type InstancesFile struct {
	Instances []InstanceEntry `json:"instances"`
}

// InstancesPath returns <datadir>/instances.json.
func InstancesPath(dataDir string) string {
	return filepath.Join(strings.TrimSpace(dataDir), instancesFileName)
}

// LoadInstances reads instances.json when present (empty slice if missing).
func LoadInstances(dataDir string) (InstancesFile, error) {
	out := InstancesFile{}
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return out, nil
	}
	if abs, err := ResolveDataDir(dataDir); err == nil && abs != "" {
		dataDir = abs
	}
	b, err := os.ReadFile(InstancesPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	normalizeInstancePaths(dataDir, &out)
	return out, nil
}

func normalizeInstancePaths(dataDir string, f *InstancesFile) {
	if f == nil {
		return
	}
	dataDir = strings.TrimSpace(dataDir)
	for i := range f.Instances {
		p := strings.TrimSpace(f.Instances[i].ConfPath)
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dataDir, filepath.Base(p))
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		f.Instances[i].ConfPath = p
	}
}

// SaveInstances writes instances.json under dataDir.
func SaveInstances(dataDir string, f InstancesFile) error {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil
	}
	if abs, err := ResolveDataDir(dataDir); err == nil && abs != "" {
		dataDir = abs
	}
	normalizeInstancePaths(dataDir, &f)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(InstancesPath(dataDir), b, 0o600)
}

// DualInstanceConfNames are per-network config files under the shared datadir.
const (
	DualMainnetConfName = "dogecoinconf.mainnet.json"
	DualTestnetConfName = "dogecoinconf.testnet.json"
)
