// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package launch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dogego/config"
)

func TestTryAcquireSpawnLock_serializes(t *testing.T) {
	dir := t.TempDir()
	if !tryAcquireSpawnLock(dir, "testnet") {
		t.Fatal("first acquire failed")
	}
	if tryAcquireSpawnLock(dir, "testnet") {
		t.Fatal("second acquire should fail while lock is fresh")
	}
	releaseSpawnLock(dir, "testnet")
	if !tryAcquireSpawnLock(dir, "testnet") {
		t.Fatal("acquire after release failed")
	}
}

func TestSpawnLockPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	got := spawnLockPath(dir, "TestNet")
	want := filepath.Join(dir, ".dogego-spawn-testnet.lock")
	if got != want {
		t.Fatalf("path %q want %q", got, want)
	}
}

func TestMissingSiblingInstances_noneWithoutFile(t *testing.T) {
	dir := t.TempDir()
	missing, err := missingSiblingInstances(dir, "mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected none, got %v", missing)
	}
}

func TestParseSpawnLockPID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock")
	if err := os.WriteFile(path, []byte("4242\n1700000000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := parseSpawnLockPID(path); got != 4242 {
		t.Fatalf("pid %d", got)
	}
}

func TestClearSpawnLockIfListening_noListen(t *testing.T) {
	dir := t.TempDir()
	if !tryAcquireSpawnLock(dir, "mainnet") {
		t.Fatal("acquire failed")
	}
	clearSpawnLockIfListening(dir, "mainnet", "127.0.0.1:1")
	if _, err := os.Stat(spawnLockPath(dir, "mainnet")); err != nil {
		t.Fatal("lock should remain when port is closed")
	}
}

func TestPeerSpawnGracePositive(t *testing.T) {
	if peerSpawnGrace <= 0 {
		t.Fatal("unexpected timing constants")
	}
	if spawnLockMaxAge < time.Minute {
		t.Fatal("spawn lock max age too short")
	}
}

func TestShouldManageDualPeers_mainnetOnly(t *testing.T) {
	dir := t.TempDir()
	inst := config.InstancesFile{
		Instances: []config.InstanceEntry{
			{Network: "mainnet", WebUI: "127.0.0.1:2013", ConfPath: filepath.Join(dir, "dogecoinconf.mainnet.json")},
			{Network: "testnet", WebUI: "127.0.0.1:2014", ConfPath: filepath.Join(dir, "dogecoinconf.testnet.json")},
		},
	}
	if err := config.SaveInstances(dir, inst); err != nil {
		t.Fatal(err)
	}
	if !shouldManageSiblingSpawns(dir, "mainnet") {
		t.Fatal("mainnet should manage sibling spawns")
	}
	if shouldManageSiblingSpawns(dir, "testnet") {
		t.Fatal("testnet should not manage sibling spawns")
	}
}

func TestShouldRegisterURLScheme_dualCoordinatorOnly(t *testing.T) {
	dir := t.TempDir()
	inst := config.InstancesFile{
		Instances: []config.InstanceEntry{
			{Network: "mainnet", WebUI: "127.0.0.1:2013", ConfPath: filepath.Join(dir, "dogecoinconf.mainnet.json")},
			{Network: "testnet", WebUI: "127.0.0.1:2014", ConfPath: filepath.Join(dir, "dogecoinconf.testnet.json")},
		},
	}
	if err := config.SaveInstances(dir, inst); err != nil {
		t.Fatal(err)
	}
	if !ShouldRegisterURLScheme(dir, "mainnet") {
		t.Fatal("mainnet should register URL scheme")
	}
	if ShouldRegisterURLScheme(dir, "testnet") {
		t.Fatal("testnet should not register URL scheme in dual mode")
	}
}
