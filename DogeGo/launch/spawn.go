// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package launch starts detached DogeGo node processes (dual mainnet + testnet).
package launch

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dogego/config"
)

var dualPeerSpawnOnce sync.Once

// TCPListenOpen reports whether host:port accepts a TCP connection.
func TCPListenOpen(hostPort string, timeout time.Duration) bool {
	hostPort = strings.TrimSpace(hostPort)
	if hostPort == "" {
		return false
	}
	if !strings.Contains(hostPort, ":") {
		hostPort = "127.0.0.1:" + hostPort
	}
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", hostPort)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// SpawnDetachedNode starts dogego node with DOGECOINCONF set (non-blocking).
func SpawnDetachedNode(confPath string, tray bool) error {
	confPath = strings.TrimSpace(confPath)
	if confPath == "" {
		return fmt.Errorf("launch: empty config path")
	}
	abs, err := filepath.Abs(confPath)
	if err != nil {
		return fmt.Errorf("launch: config path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("launch: config %s: %w", abs, err)
	}
	return spawnDetachedNodePlatform(abs, tray)
}

// ShouldManageDualPeers reports whether this process should start peer instances.
// In dual mode only the primary entry in instances.json (mainnet) coordinates peers.
func ShouldManageDualPeers(dataDir, currentNetwork string) bool {
	return shouldManageSiblingSpawns(dataDir, currentNetwork)
}

// ShouldRegisterURLScheme reports whether this process should register dogecoin://.
// In dual mode only the coordinator registers the handler (avoids per-peer churn).
func ShouldRegisterURLScheme(dataDir, currentNetwork string) bool {
	dataDir = resolveAbsDataDir(dataDir)
	inst, err := config.LoadInstances(dataDir)
	if err != nil || len(inst.Instances) < 2 {
		return true
	}
	return shouldManageSiblingSpawns(dataDir, currentNetwork)
}

// StartDualPeersOnce spawns missing peer instances after the coordinator dashboard is listening.
// Runs at most once per process; duplicate spawns are blocked by per-network lock files.
func StartDualPeersOnce(dataDir, currentNetwork string, tray bool) {
	dualPeerSpawnOnce.Do(func() {
		startDualPeers(dataDir, currentNetwork, tray)
	})
}

func startDualPeers(dataDir, currentNetwork string, tray bool) {
	dataDir = resolveAbsDataDir(dataDir)
	if !shouldManageSiblingSpawns(dataDir, currentNetwork) {
		return
	}
	time.Sleep(peerSpawnGrace)
	missing, err := missingSiblingInstances(dataDir, currentNetwork)
	if err != nil || len(missing) == 0 {
		return
	}
	for _, e := range missing {
		netSlug := strings.ToLower(strings.TrimSpace(e.Network))
		if netSlug == "" {
			continue
		}
		if TCPListenOpen(e.WebUI, 400*time.Millisecond) {
			clearSpawnLockIfListening(dataDir, netSlug, e.WebUI)
			continue
		}
		confPath := strings.TrimSpace(e.ConfPath)
		if confPath == "" {
			continue
		}
		if _, err := os.Stat(confPath); err != nil {
			fmt.Fprintf(os.Stderr, "DogeGo: skip %s peer (config missing: %s)\n", e.Network, confPath)
			continue
		}
		if !tryAcquireSpawnLock(dataDir, netSlug) {
			continue
		}
		if err := SpawnDetachedNode(confPath, false); err != nil {
			releaseSpawnLock(dataDir, netSlug)
			fmt.Fprintf(os.Stderr, "DogeGo: could not start %s instance: %v\n", e.Network, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "DogeGo: started %s peer (config %s, web UI %s)\n", e.Network, confPath, e.WebUI)
		go waitClearSpawnLock(dataDir, netSlug, e.WebUI)
	}
}

func waitClearSpawnLock(dataDir, network, webUI string) {
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if TCPListenOpen(webUI, 400*time.Millisecond) {
			clearSpawnLockIfListening(dataDir, network, webUI)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}
