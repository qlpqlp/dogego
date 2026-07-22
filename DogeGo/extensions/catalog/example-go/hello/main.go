// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

// hello-ext is the DogeGo hello-world subprocess extension (line JSON-RPC on stdin/stdout).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var req struct {
			ID     uint64            `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if json.Unmarshal(line, &req) != nil {
			writeErr(0, "bad json")
			continue
		}
		switch req.Method {
		case "dogego_on_enable":
			writeOK(req.ID, map[string]string{"status": "ready"})
		case "dogego_on_disable":
			writeOK(req.ID, map[string]string{"status": "bye"})
			return
		case "ping":
			writeOK(req.ID, "pong")
		case "info", "ui_status":
			writeOK(req.ID, map[string]interface{}{
				"extension": "example.go",
				"status":    "ready",
				"ui": map[string]interface{}{
					"panel_title": "Hello extension",
					"subtitle":    "Demo of modern host-rendered forms, switches, tables, and charts.",
					"layout":      "workspace",
					"status_chips": []map[string]interface{}{
						{"id": "alive", "label": "Status", "value": "Ready", "tone": "ok", "icon": "check_circle"},
					},
					"nav": []map[string]interface{}{
						{"id": "home", "label": "Home", "icon": "home"},
						{"id": "tools", "label": "Tools", "icon": "construction"},
						{"id": "settings", "label": "Settings", "icon": "tune"},
					},
					"sections": map[string]interface{}{
						"home": map[string]interface{}{
							"title": "Overview",
							"lead":  "Showcase widgets used by Doginals / ZK L2 / BBPoW.",
							"widgets": []map[string]interface{}{
								{"type": "stats", "items": []map[string]interface{}{
									{"label": "Pings", "value": "∞", "icon": "network_ping"},
									{"label": "UI kit", "value": "v2", "icon": "palette"},
								}},
								{
									"type":    "progress",
									"label":   "Demo progress",
									"percent": 72,
									"value":   "72%",
								},
								{
									"type":   "metric_chart",
									"title":  "Sample metrics",
									"chart":  "line",
									"labels": []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
									"series": []map[string]interface{}{
										{"label": "Calls", "color": "#c2a633", "data": []float64{2, 5, 3, 8, 6}},
									},
								},
								{
									"type":      "table",
									"title":     "Sample rows",
									"search":    true,
									"page_size": 5,
									"columns": []map[string]interface{}{
										{"key": "name", "label": "Name"},
										{"key": "value", "label": "Value"},
									},
									"rows": []map[string]interface{}{
										{"name": "ping", "value": "pong"},
										{"name": "switch", "value": "on"},
										{"name": "table", "value": "search + scroll"},
										{"name": "chart", "value": "Chart.js"},
										{"name": "menu", "value": "collapse"},
										{"name": "mobile", "value": "friendly"},
									},
								},
							},
							"quick_actions": []map[string]interface{}{
								{"id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh"},
								{"id": "ping", "label": "Ping", "method": "ping", "icon": "network_ping"},
							},
						},
						"tools": map[string]interface{}{
							"title": "Tools",
							"lead":  "Host-rendered forms only (no HTML from the extension).",
							"tools": []map[string]interface{}{
								{"id": "ping", "label": "Ping", "method": "ping", "icon": "network_ping", "hint": "Returns pong."},
							},
						},
						"settings": map[string]interface{}{
							"title": "Settings & security",
							"lead":  "Demo toggles (not persisted). Real extensions store prefs in data/ so upgrades keep them.",
							"widgets": []map[string]interface{}{
								{
									"type":  "callout",
									"tone":  "neutral",
									"icon":  "folder_special",
									"title": "Data survives upgrades",
									"body":  "Zip install restores extensions/<id>/data/ after package replace.",
								},
							},
							"tools": []map[string]interface{}{
								{
									"id": "demo_toggles", "label": "Demo preferences", "method": "ping", "icon": "tune", "run_label": "Ping to confirm",
									"fields": []map[string]interface{}{
										{"name": "notify", "label": "Enable notifications (demo)", "type": "switch", "default": "true"},
										{"name": "verbose", "label": "Verbose results (demo)", "type": "switch", "default": "false"},
									},
								},
							},
						},
					},
				},
			})
		default:
			writeErr(req.ID, "unknown method "+req.Method)
		}
	}
	if err := sc.Err(); err != nil {
		writeErr(0, "stdin error: "+err.Error())
	}
}

func writeOK(id uint64, result interface{}) {
	raw, _ := json.Marshal(map[string]interface{}{"id": id, "result": result})
	fmt.Println(string(raw))
}

func writeErr(id uint64, msg string) {
	raw, _ := json.Marshal(map[string]interface{}{"id": id, "error": msg})
	fmt.Println(string(raw))
}
