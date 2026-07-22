// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

//go:embed trayicon.png
var trayIconPNG []byte // Official Dogecoin logo (32×32); source: ui/static/dogecoin.svg

//go:embed trayicon_testnet.png
var trayIconTestnetPNG []byte // Testnet logo (32×32); source: ui/static/dogecoin_testnet.svg

// TrayUpdateInfo is a snapshot for tray menu labels (no dependency on version package).
type TrayUpdateInfo struct {
	Available      bool
	Dismissed      bool
	Current        string
	Latest         string
	ReleaseURL     string
	DownloadURL    string
	DirectDownload bool
	CheckError     string
}

// TrayConfig configures the system tray while the node is running.
type TrayConfig struct {
	Title        string
	Tooltip      string
	Version      string
	Network      string
	DashboardURL string
	PeerLinks    []TrayPeerLink

	OnOpen          func()
	OnOpenConsole   func()
	OnOpenLogs      func()
	QuitLabel       string
	OnShutdown      func()
	OnCheckUpdates  func()
	OnDownloadUpdate func() (path string, err error)
	OnApplyUpdate   func() error
	OnOpenRelease   func()
	OnDismissUpdate func() error

	// UpdateStatus returns the latest cached update check (may be nil).
	UpdateStatus func() TrayUpdateInfo
}

// TraySupported reports whether a system tray is available on this OS build.
func TraySupported() bool {
	return platformTraySupported()
}

// StartTray runs the system tray event loop (blocks until Quit).
func StartTray(cfg TrayConfig) error {
	if !TraySupported() {
		return fmt.Errorf("system tray is not supported on this platform")
	}
	if cfg.OnShutdown == nil {
		return fmt.Errorf("tray: OnShutdown required")
	}
	if cfg.OnOpen == nil {
		cfg.OnOpen = func() {}
	}
	if cfg.OnOpenConsole == nil {
		cfg.OnOpenConsole = cfg.OnOpen
	}
	if cfg.OnOpenLogs == nil {
		cfg.OnOpenLogs = cfg.OnOpenConsole
	}
	if cfg.Title == "" {
		cfg.Title = "DogeGo"
	}
	if cfg.Tooltip == "" {
		cfg.Tooltip = TrayBaseTooltip(TrayNetworkLabel(cfg.Network))
	}
	if strings.TrimSpace(cfg.Version) == "" {
		cfg.Version = "unknown"
	}
	return runTray(cfg, trayIconBytesForNetwork(cfg.Network))
}

// TrayNetworkLabel returns a short network slug for tray text (mainnet / testnet).
func TrayNetworkLabel(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "mainnet", "main":
		return "mainnet"
	case "testnet", "reboottestnet":
		return "testnet"
	default:
		if network == "" {
			return "node"
		}
		return strings.ToLower(strings.TrimSpace(network))
	}
}

// TrayBaseTooltip builds the default systray hover text for a network.
func TrayBaseTooltip(netLabel string) string {
	netLabel = strings.TrimSpace(netLabel)
	if netLabel == "" || netLabel == "node" {
		return "DogeGo Node"
	}
	return "DogeGo " + netLabel + " node"
}

// OpenDashboardTab opens the web UI at an optional hash route (e.g. console, settings).
func OpenDashboardTab(baseURL, hash string) {
	u := strings.TrimSpace(baseURL)
	if u == "" {
		fmt.Fprintln(os.Stderr, "DogeGo tray: web UI disabled (nowebui)")
		return
	}
	hash = strings.TrimPrefix(strings.TrimSpace(hash), "#")
	if hash != "" {
		if strings.Contains(u, "#") {
			u = strings.Split(u, "#")[0]
		}
		u = strings.TrimSuffix(u, "/") + "/#" + hash
	}
	OpenURLForce(u)
}

func trayVersionLabel(version, network string, upd TrayUpdateInfo) string {
	v := strings.TrimSpace(version)
	if v == "" {
		v = "unknown"
	}
	netLabel := TrayNetworkLabel(network)
	label := "DogeGo " + v + " (" + netLabel + ")"
	if upd.Available && !upd.Dismissed && upd.Latest != "" {
		label += " · update " + upd.Latest
	}
	return label
}

func trayTooltip(base string, upd TrayUpdateInfo) string {
	tip := strings.TrimSpace(base)
	if tip == "" {
		tip = "DogeGo Node"
	}
	if len(tip) > 120 {
		tip = tip[:117] + "..."
	}
	if upd.Available && !upd.Dismissed && upd.Latest != "" {
		extra := " - Update " + upd.Latest
		if len(tip)+len(extra) > 120 {
			return tip
		}
		return tip + extra
	}
	return tip
}
