// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package netfw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Mode controls when DogeGo attempts to add OS firewall rules for P2P.
type Mode int

const (
	ModeNever Mode = iota
	ModeAuto
	ModeAlways
)

// ParseMode interprets dogecoinconf.json / -firewall values (empty → auto).
func ParseMode(s string) Mode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "always", "1", "yes", "true", "on":
		return ModeAlways
	case "never", "0", "no", "false", "off":
		return ModeNever
	default:
		return ModeAuto
	}
}

func (m Mode) String() string {
	switch m {
	case ModeAlways:
		return "always"
	case ModeNever:
		return "never"
	default:
		return "auto"
	}
}

// Config describes the P2P endpoints to allow through the host firewall.
type Config struct {
	AppName  string // e.g. "DogeGo"
	ExePath  string // absolute path to dogego binary
	Port     int    // chain P2P port (22556 mainnet, 44556 testnet)
	Inbound  bool   // allow TCP inbound on Port (when listening)
	Outbound bool   // allow outbound for ExePath (dials to peers)
	Mode     Mode
	// Elevate requests a one-time OS admin prompt when rules are missing (auto on Windows).
	Elevate bool
}

// Result reports what Ensure did.
type Result struct {
	OK          bool
	AlreadyOK   bool
	NeedsAdmin  bool
	Applied     []string
	Platform    string
	UserMessage string
	Err         error
}

// DefaultConfig fills AppName and resolves ExePath when empty.
func DefaultConfig(port int, inbound, outbound bool, mode Mode) Config {
	exe, _ := os.Executable()
	if exe != "" {
		exe, _ = filepath.Abs(exe)
	}
	return Config{
		AppName:  "DogeGo",
		ExePath:  exe,
		Port:     port,
		Inbound:  inbound,
		Outbound: outbound,
		Mode:     mode,
		Elevate:  mode == ModeAuto || mode == ModeAlways,
	}
}

// Ensure adds platform firewall rules when Mode is not Never.
func Ensure(cfg Config) Result {
	if cfg.Mode == ModeNever {
		return Result{OK: true, Platform: platformName()}
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return Result{Err: fmt.Errorf("netfw: invalid port %d", cfg.Port)}
	}
	if cfg.ExePath == "" {
		var err error
		cfg.ExePath, err = os.Executable()
		if err != nil {
			return Result{Err: fmt.Errorf("netfw: executable path: %w", err)}
		}
	}
	if abs, err := filepath.Abs(cfg.ExePath); err == nil {
		cfg.ExePath = abs
	}
	if cfg.AppName == "" {
		cfg.AppName = "DogeGo"
	}
	if Present(cfg) {
		return Result{OK: true, AlreadyOK: true, Platform: platformName(), UserMessage: "firewall rules already present"}
	}
	if cfg.Mode == ModeAuto && !cfg.Inbound && !cfg.Outbound {
		return Result{OK: true, Platform: platformName()}
	}
	res := ensurePlatform(cfg)
	PublishResult(cfg, res)
	return res
}

// Present reports whether expected rules already exist.
func Present(cfg Config) bool {
	if cfg.Port <= 0 {
		return true
	}
	return presentPlatform(cfg)
}

// ManualInstructions returns copy-paste commands when automatic setup needs admin rights.
func ManualInstructions(cfg Config) string {
	return manualPlatform(cfg)
}
