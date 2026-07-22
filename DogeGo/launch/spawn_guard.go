// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dogego/config"
)

const (
	peerSpawnGrace        = 1500 * time.Millisecond
	spawnLockMaxAge       = 3 * time.Minute
	spawnLockFileTemplate = ".dogego-spawn-%s.lock"
)

func resolveAbsDataDir(dataDir string) string {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return ""
	}
	if abs, err := config.ResolveDataDir(dataDir); err == nil && abs != "" {
		return abs
	}
	if abs, err := filepath.Abs(dataDir); err == nil {
		return abs
	}
	return dataDir
}

func spawnLockPath(dataDir, network string) string {
	netSlug := strings.ToLower(strings.TrimSpace(network))
	if netSlug == "" {
		netSlug = "unknown"
	}
	return filepath.Join(resolveAbsDataDir(dataDir), fmt.Sprintf(spawnLockFileTemplate, netSlug))
}

// tryAcquireSpawnLock returns true when this process may spawn the given network.
func tryAcquireSpawnLock(dataDir, network string) bool {
	dataDir = resolveAbsDataDir(dataDir)
	if dataDir == "" {
		return false
	}
	path := spawnLockPath(dataDir, network)
	if info, err := os.Stat(path); err == nil {
		if time.Since(info.ModTime()) < spawnLockMaxAge {
			return false
		}
		_ = os.Remove(path)
	} else if !os.IsNotExist(err) {
		return false
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false
	}
	_, _ = fmt.Fprintf(f, "%d\n%d\n", os.Getpid(), time.Now().Unix())
	_ = f.Close()
	return true
}

func releaseSpawnLock(dataDir, network string) {
	_ = os.Remove(spawnLockPath(dataDir, network))
}

func clearSpawnLockIfListening(dataDir, network, webUI string) {
	if TCPListenOpen(webUI, 400*time.Millisecond) {
		releaseSpawnLock(dataDir, network)
	}
}

// shouldManageSiblingSpawns reports whether this process should start peer instances.
func shouldManageSiblingSpawns(dataDir, currentNetwork string) bool {
	dataDir = resolveAbsDataDir(dataDir)
	inst, err := config.LoadInstances(dataDir)
	if err != nil || len(inst.Instances) < 2 {
		return false
	}
	primary := strings.ToLower(strings.TrimSpace(inst.Instances[0].Network))
	cur := strings.ToLower(strings.TrimSpace(currentNetwork))
	return primary != "" && primary == cur
}

func missingSiblingInstances(dataDir, currentNetwork string) ([]config.InstanceEntry, error) {
	dataDir = resolveAbsDataDir(dataDir)
	inst, err := config.LoadInstances(dataDir)
	if err != nil {
		return nil, err
	}
	if len(inst.Instances) < 2 {
		return nil, nil
	}
	cur := strings.ToLower(strings.TrimSpace(currentNetwork))
	var missing []config.InstanceEntry
	for _, e := range inst.Instances {
		netSlug := strings.ToLower(strings.TrimSpace(e.Network))
		if netSlug == "" || netSlug == cur {
			continue
		}
		if TCPListenOpen(e.WebUI, 400*time.Millisecond) {
			clearSpawnLockIfListening(dataDir, netSlug, e.WebUI)
			continue
		}
		missing = append(missing, e)
	}
	return missing, nil
}

// parseSpawnLockPID reads the spawning PID from a lock file (best-effort).
func parseSpawnLockPID(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	line, _, _ := strings.Cut(string(b), "\n")
	pid, _ := strconv.Atoi(strings.TrimSpace(line))
	return pid
}
