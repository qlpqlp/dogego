// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import "testing"

func TestDefaultWorkspaceUI(t *testing.T) {
	ui := DefaultWorkspaceUI("Hello", "sub", []UITool{{ID: "ping", Label: "Ping", Method: "ping", Icon: "network_ping"}})
	if ui["layout"] != "workspace" {
		t.Fatalf("%v", ui["layout"])
	}
	nav, _ := ui["nav"].([]map[string]interface{})
	if len(nav) < 3 {
		t.Fatalf("nav %#v", nav)
	}
	sec, _ := ui["sections"].(map[string]interface{})
	if sec["tools"] == nil || sec["settings"] == nil {
		t.Fatalf("sections %#v", sec)
	}
}

func TestToolsFromManifestSkipsInfo(t *testing.T) {
	m := Manifest{
		RPC: []RPCMethod{{Name: "info"}, {Name: "ping", Help: "pong"}},
	}
	tools := ToolsFromManifest(m)
	if len(tools) != 1 || tools[0].Method != "ping" {
		t.Fatalf("%+v", tools)
	}
}
