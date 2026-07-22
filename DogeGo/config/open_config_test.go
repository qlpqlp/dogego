// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOpenConfigDualPort(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, DualMainnetConfName)
	testPath := filepath.Join(dir, DualTestnetConfName)
	main := File{DataDir: dir, Network: "mainnet", WebUI: DualMainnetWebUI}
	test := File{DataDir: dir, Network: "testnet", WebUI: DualTestnetWebUI}
	if err := Save(mainPath, main); err != nil {
		t.Fatal(err)
	}
	if err := Save(testPath, test); err != nil {
		t.Fatal(err)
	}
	inst := InstancesFile{
		Instances: []InstanceEntry{
			{Network: "mainnet", WebUI: DualMainnetWebUI, ConfPath: mainPath},
			{Network: "testnet", WebUI: DualTestnetWebUI, ConfPath: testPath},
		},
	}
	if err := SaveInstances(dir, inst); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGECOINCONF", mainPath)

	f, path := ResolveOpenConfig("http://127.0.0.1:2014/")
	if path != testPath {
		t.Fatalf("testnet path %q want %q", path, testPath)
	}
	if f.Network != "testnet" {
		t.Fatalf("network %q want testnet", f.Network)
	}

	f, path = ResolveOpenConfig("http://127.0.0.1:2013/")
	if path != mainPath {
		t.Fatalf("mainnet path %q want %q", path, mainPath)
	}
	if f.Network != "mainnet" {
		t.Fatalf("network %q want mainnet", f.Network)
	}
}

func TestResolveOpenConfigEmptyUsesLoadFirst(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(`{"network":"mainnet","webui":"127.0.0.1:2013"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGECOINCONF", path)
	f, got := ResolveOpenConfig("")
	if got != path || f.Network != "mainnet" {
		t.Fatalf("got %+v %q", f, got)
	}
}
