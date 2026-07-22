// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"path/filepath"
	"testing"

	"dogego/config"
)

func TestPeerTrayLinksDual(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, config.DualMainnetConfName)
	testPath := filepath.Join(dir, config.DualTestnetConfName)
	inst := config.InstancesFile{
		Instances: []config.InstanceEntry{
			{Network: "mainnet", WebUI: config.DualMainnetWebUI, ConfPath: mainPath, Label: "Mainnet"},
			{Network: "testnet", WebUI: config.DualTestnetWebUI, ConfPath: testPath, Label: "Testnet"},
		},
	}
	if err := config.SaveInstances(dir, inst); err != nil {
		t.Fatal(err)
	}
	links := PeerTrayLinks(dir, "mainnet")
	if len(links) != 1 || links[0].URL != "http://localhost:2014/" {
		t.Fatalf("mainnet peer links: %+v", links)
	}
	links = PeerTrayLinks(dir, "testnet")
	if len(links) != 1 || links[0].URL != "http://localhost:2013/" {
		t.Fatalf("testnet peer links: %+v", links)
	}
}
