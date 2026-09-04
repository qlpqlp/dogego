// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dogeos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const configFileName = "config.json"

// Config is persisted under the extension data directory.
type Config struct {
	NetworkID      string `json:"network_id"`
	CustomRPCURL   string `json:"custom_rpc_url,omitempty"`
	PollSeconds    int    `json:"poll_seconds"`
	MetricsEnabled bool   `json:"metrics_enabled"`
}

func DefaultConfig() Config {
	return Config{
		NetworkID:      DefaultNetworkID(),
		PollSeconds:    15,
		MetricsEnabled: true,
	}
}

type configStore struct {
	mu   sync.Mutex
	path string
	cfg  Config
}

func loadConfig(dataDir string) *configStore {
	s := &configStore{
		path: filepath.Join(strings.TrimSpace(dataDir), configFileName),
		cfg:  DefaultConfig(),
	}
	raw, err := os.ReadFile(s.path)
	if err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &s.cfg)
	}
	s.normalize()
	return s
}

func (s *configStore) normalize() {
	s.cfg.NetworkID = strings.TrimSpace(s.cfg.NetworkID)
	if s.cfg.NetworkID == "" {
		s.cfg.NetworkID = DefaultNetworkID()
	}
	if _, ok := FindNetwork(s.cfg.NetworkID); !ok {
		s.cfg.NetworkID = DefaultNetworkID()
	}
	s.cfg.CustomRPCURL = strings.TrimSpace(s.cfg.CustomRPCURL)
	if s.cfg.PollSeconds < 5 {
		s.cfg.PollSeconds = 5
	}
	if s.cfg.PollSeconds > 300 {
		s.cfg.PollSeconds = 300
	}
}

func (s *configStore) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

func (s *configStore) Set(cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
	s.normalize()
	return s.saveLocked()
}

func (s *configStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}

// EffectiveRPC returns custom RPC if set, else the selected network RPC.
func (s *configStore) EffectiveRPC() (NetworkProfile, string, error) {
	cfg := s.Get()
	n, ok := FindNetwork(cfg.NetworkID)
	if !ok {
		n, _ = FindNetwork(DefaultNetworkID())
	}
	rpc := strings.TrimRight(n.RPCURL, "/")
	if cfg.CustomRPCURL != "" {
		rpc = strings.TrimRight(cfg.CustomRPCURL, "/")
	}
	if !n.Available && cfg.CustomRPCURL == "" {
		return n, rpc, errNetworkUnavailable(n)
	}
	if rpc == "" {
		return n, rpc, errNetworkUnavailable(n)
	}
	return n, rpc + "/", nil
}

func errNetworkUnavailable(n NetworkProfile) error {
	return &NetworkUnavailableError{Network: n}
}

// NetworkUnavailableError is returned when the selected profile has no live RPC.
type NetworkUnavailableError struct {
	Network NetworkProfile
}

func (e *NetworkUnavailableError) Error() string {
	name := e.Network.Name
	if name == "" {
		name = e.Network.ID
	}
	if e.Network.Notes != "" {
		return name + " is not available yet. " + e.Network.Notes
	}
	return name + " is not available yet"
}
