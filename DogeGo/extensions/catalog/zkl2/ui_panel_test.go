// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"testing"
)

func TestBuildUIPanel(t *testing.T) {
	info := map[string]interface{}{
		"p2p_protocol":     "zkproof-v1",
		"zkproof_v1_peers": 2,
		"l2_tip_height":    42,
		"groth16": map[string]interface{}{
			"pairing_enabled":  true,
			"default_vk_bytes": 432,
		},
	}
	ui := buildUIPanel(info)
	if ui["panel_title"] != "ZK L2" {
		t.Fatalf("panel_title %#v", ui["panel_title"])
	}
	if ui["layout"] != "workspace" {
		t.Fatal("expected workspace layout")
	}
	chips, _ := ui["status_chips"].([]map[string]interface{})
	if len(chips) < 3 {
		t.Fatalf("expected status chips, got %d", len(chips))
	}
	nav, _ := ui["nav"].([]map[string]interface{})
	if len(nav) < 3 {
		t.Fatal("expected nav")
	}
	sections, _ := ui["sections"].(map[string]interface{})
	home, _ := sections["home"].(map[string]interface{})
	if home == nil {
		t.Fatal("expected home section")
	}
	widgets, _ := home["widgets"].([]map[string]interface{})
	if len(widgets) == 0 {
		t.Fatal("expected home widgets")
	}
	info["proof_total"] = 3
	info["recent_proofs"] = []map[string]interface{}{
		{"proof_hash": "aa", "transaction_id": "bb", "block_hash": "cc", "block_height": 1},
	}
	ui2 := buildUIPanel(info)
	sec2, _ := ui2["sections"].(map[string]interface{})
	home2, _ := sec2["home"].(map[string]interface{})
	w2, _ := home2["widgets"].([]map[string]interface{})
	if len(w2) < 2 {
		t.Fatalf("expected stats + proof_list widgets, got %d", len(w2))
	}
}

func TestDefaultManifestUIPanel(t *testing.T) {
	m := DefaultManifest()
	if !m.HasPermission("ui_panel") {
		t.Fatal("expected ui_panel permission")
	}
	if m.UI.StatusMethod != "info" {
		t.Fatalf("status method %q", m.UI.StatusMethod)
	}
}

func TestInstallDefaultDemoVK(t *testing.T) {
	dir := t.TempDir()
	n, err := InstallDefaultDemoVK(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n <= 0 {
		t.Fatalf("bytes %d", n)
	}
	if len(vkBytes("")) == 0 {
		t.Fatal("expected loaded default vk")
	}
}
