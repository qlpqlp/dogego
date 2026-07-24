// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package radiodoge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	DefaultBaseURL     = "http://192.168.4.1"
	DefaultPollSeconds = 15
	configFileName     = "config.json"
)

// Config is persisted under the extension data directory.
type Config struct {
	BaseURL             string `json:"base_url"`
	Enabled             bool   `json:"enabled"`
	PreferRadioOffline  bool   `json:"prefer_radio_offline"`
	ForceRadio          bool   `json:"force_radio"`
	AutoRelayInbound    bool   `json:"auto_relay_inbound"`
	ConfirmViaLogs      bool   `json:"confirm_via_logs"`
	PollSeconds         int    `json:"poll_seconds"`
	InternetProbeURL    string `json:"internet_probe_url"`
	GatewayType         string `json:"gateway_type"`
	GatewayIP           string `json:"gateway_ip"`
	GatewayPort         string `json:"gateway_port"`
	GatewayUser         string `json:"gateway_user"`
	GatewayPassword     string `json:"gateway_password"`
	GatewayEndpoint     string `json:"gateway_endpoint"`
	DirectTargetAddress string `json:"direct_target_address"`
}

func DefaultConfig() Config {
	return Config{
		BaseURL:            DefaultBaseURL,
		Enabled:            true,
		PreferRadioOffline: true,
		ForceRadio:         false,
		AutoRelayInbound:   true,
		ConfirmViaLogs:     true,
		PollSeconds:        DefaultPollSeconds,
		InternetProbeURL:   "http://1.1.1.1",
		GatewayType:        "custom",
		GatewayPort:        "22555",
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
	if strings.TrimSpace(s.cfg.BaseURL) == "" {
		s.cfg.BaseURL = DefaultBaseURL
	}
	s.cfg.BaseURL = strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/")
	if s.cfg.PollSeconds < 5 {
		s.cfg.PollSeconds = DefaultPollSeconds
	}
	if s.cfg.PollSeconds > 300 {
		s.cfg.PollSeconds = 300
	}
	if strings.TrimSpace(s.cfg.InternetProbeURL) == "" {
		s.cfg.InternetProbeURL = "http://1.1.1.1"
	}
}

func (s *configStore) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

func (s *configStore) Update(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.cfg)
	s.normalize()
	return s.saveLocked()
}

func (s *configStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}
