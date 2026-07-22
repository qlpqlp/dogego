// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"fmt"
)

// buildUIPanel returns extension-owned dashboard copy for the Settings panel.
func buildUIPanel(info map[string]interface{}) map[string]interface{} {
	gro, _ := info["groth16"].(map[string]interface{})
	pairing, _ := gro["pairing_enabled"].(bool)
	peerN, _ := asUint64(info["zkproof_v1_peers"])
	proofN, _ := asUint64(info["proof_total"])
	tipH, _ := asUint64(info["l2_tip_height"])

	chips := []map[string]interface{}{
		{"id": "protocol", "label": "Protocol", "value": "zkproof-v1", "tone": "neutral", "icon": "hub"},
	}
	if peerN == 1 {
		chips = append(chips, map[string]interface{}{"id": "peers", "label": "Overlay peers", "value": "1 connected", "tone": "ok", "icon": "group"})
	} else {
		chips = append(chips, map[string]interface{}{"id": "peers", "label": "Overlay peers", "value": fmt.Sprintf("%d connected", peerN), "tone": peerTone(peerN), "icon": "group"})
	}
	if pairing {
		bytes, _ := asUint64(gro["default_vk_bytes"])
		chips = append(chips, map[string]interface{}{"id": "groth16", "label": "Groth16 pairing", "value": fmt.Sprintf("On · %d B VK", bytes), "tone": "ok", "icon": "verified"})
	} else {
		chips = append(chips, map[string]interface{}{"id": "groth16", "label": "Groth16 pairing", "value": "Off", "tone": "warn", "icon": "verified_user"})
	}

	subtitle := "Decentralized zkproof overlay syncs proofs with other DogeGo nodes on P2P zkproof-v1."
	if tipH > 0 {
		subtitle = fmt.Sprintf("L2 tip height %d. %s", tipH, subtitle)
	}

	return map[string]interface{}{
		"panel_title":  "ZK L2",
		"subtitle":     subtitle,
		"status_chips": chips,
		"layout":       "workspace",
		"nav": []map[string]interface{}{
			{"id": "home", "label": "Home", "icon": "home"},
			{"id": "proofs", "label": "Proofs", "icon": "verified"},
			{"id": "create", "label": "Create", "icon": "auto_awesome"},
			{"id": "tools", "label": "Tools", "icon": "construction"},
			{"id": "settings", "label": "Settings", "icon": "tune"},
		},
		"sections": map[string]interface{}{
			"home": map[string]interface{}{
				"title":   "Overview",
				"lead":    "Metrics and recent proofs. Use the menu for create flows and advanced tools.",
				"widgets": zkl2DashboardWidgets(info),
				"quick_actions": []map[string]interface{}{
					{"id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh"},
					{"id": "listproofs", "label": "List proofs", "method": "listproofs", "icon": "list", "params": []interface{}{"", float64(10)}},
				},
			},
			"proofs": map[string]interface{}{
				"title":   "Indexed proofs",
				"lead":    "Searchable table with infinite scroll. Load more from the node when needed.",
				"widgets": proofTableWidgets(info),
				"tools": []map[string]interface{}{
					{
						"id": "listproofs", "label": "Search proofs", "method": "listproofs", "icon": "search",
						"fields": []map[string]interface{}{
							{"name": "block_hash", "label": "Block hash (optional)", "type": "text", "placeholder": "64-char hex"},
							{"name": "limit", "label": "Limit", "type": "number", "default": "50"},
						},
					},
					{
						"id": "getproof", "label": "Get proof by hash", "method": "getproof", "icon": "fingerprint", "advanced": true,
						"fields": []map[string]interface{}{
							{"name": "proof_hash", "label": "Proof hash", "type": "text", "placeholder": "64-char hex"},
						},
					},
				},
			},
			"create": map[string]interface{}{
				"title": "Create a proof",
				"lead":  "Step through generate then submit. Wallet signing uses authenticated wallet_rpc (unlock in dashboard).",
				"wizards": []map[string]interface{}{
					{
						"id": "gen_wizard", "title": "Generate proof", "method": "generateproof", "icon": "auto_awesome",
						"finish_label": "Generate",
						"params_as":    "object",
						"steps": []map[string]interface{}{
							{
								"id": "kind", "title": "Kind",
								"fields": []map[string]interface{}{
									{"name": "proof_kind", "label": "Proof kind", "type": "select", "default": "commitment",
										"options": []map[string]string{
											{"value": "commitment", "label": "Commitment (SHA256)", "icon": "fingerprint"},
											{"value": "groth16", "label": "Groth16", "icon": "verified"},
										}},
									{"name": "demo_groth16", "label": "Demo Groth16 (test only)", "type": "switch", "default": "false",
										"hint": "Uses the bundled demo verifying key."},
								},
							},
							{
								"id": "payload", "title": "Payload",
								"fields": []map[string]interface{}{
									{"name": "payload", "label": "Text payload", "type": "textarea", "placeholder": "much wow"},
									{"name": "payload_encoding", "label": "Encoding", "type": "select", "default": "text",
										"options": []map[string]string{
											{"value": "text", "label": "Text", "icon": "notes"},
											{"value": "base64", "label": "Base64", "icon": "code"},
										}},
								},
							},
						},
					},
				},
				"quick_actions": []map[string]interface{}{
					{"id": "install_vk", "label": "Install demo VK", "method": "installdefaultvk", "icon": "key"},
				},
			},
			"tools": map[string]interface{}{
				"title": "Advanced tools",
				"lead":  "Collapsible forms for JSON workflows.",
				"tools": zkl2DashboardTools(),
			},
			"settings": map[string]interface{}{
				"title": "Wallet & data",
				"lead":  "Proof DB and verifying keys live under data/ and survive zip upgrades.",
				"widgets": []map[string]interface{}{
					{
						"type":  "callout",
						"tone":  "ok",
						"icon":  "folder_special",
						"title": "Upgrade-safe storage",
						"body":  "Install/Update preserves extensions/dogego.zkl2/data/ (proofs + vk/). Keys never leave the node wallet.",
					},
					{
						"type": "stats",
						"items": []map[string]interface{}{
							{"label": "Proofs", "value": fmt.Sprintf("%d", proofN), "icon": "verified"},
							{"label": "Peers", "value": fmt.Sprintf("%d", peerN), "icon": "hub"},
							{"label": "L2 tip", "value": fmt.Sprintf("%d", tipH), "icon": "layers"},
						},
					},
				},
				"quick_actions": []map[string]interface{}{
					{"id": "refresh", "label": "Refresh status", "method": "info", "icon": "refresh"},
				},
			},
		},
	}
}

func peerTone(n uint64) string {
	if n > 0 {
		return "ok"
	}
	return "warn"
}

func proofTableWidgets(info map[string]interface{}) []map[string]interface{} {
	rows := proofRows(info)
	out := []map[string]interface{}{{
		"type":               "table",
		"title":              "Proof index",
		"search":             true,
		"search_placeholder": "Search hash / tx…",
		"page_size":          12,
		"columns": []map[string]interface{}{
			{"key": "proof_hash", "label": "Proof", "mono": true},
			{"key": "height", "label": "Height"},
			{"key": "tx", "label": "Tx", "mono": true},
		},
		"rows": rows,
		"load_more": map[string]interface{}{
			"method":            "listproofs",
			"params":            []interface{}{"", float64(50)},
			"limit_param_index": 1,
			"map_rows":          "proofs",
		},
	}}
	for _, w := range zkl2DashboardWidgets(info) {
		if t, _ := w["type"].(string); t == "proof_list" {
			out = append(out, w)
		}
	}
	return out
}

func proofRows(info map[string]interface{}) []map[string]interface{} {
	var rows []map[string]interface{}
	add := func(p map[string]interface{}) {
		if p == nil {
			return
		}
		ph, _ := p["proof_hash"].(string)
		if ph == "" {
			ph, _ = p["proofHash"].(string)
		}
		txid, _ := p["transaction_id"].(string)
		if txid == "" {
			txid, _ = p["transactionId"].(string)
		}
		rows = append(rows, map[string]interface{}{
			"proof_hash": shortHex(ph, 16),
			"height":     p["block_height"],
			"tx":         shortHex(txid, 12),
		})
	}
	if recent, ok := info["recent_proofs"].([]map[string]interface{}); ok {
		for _, p := range recent {
			add(p)
		}
		return rows
	}
	if raw, ok := info["recent_proofs"].([]interface{}); ok {
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				add(m)
			}
		}
	}
	return rows
}

func shortHex(s string, n int) string {
	s = fmt.Sprintf("%v", s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func zkl2DashboardWidgets(info map[string]interface{}) []map[string]interface{} {
	widgets := []map[string]interface{}{}
	statItems := []map[string]interface{}{}
	proofN, _ := asUint64(info["proof_total"])
	tipH, _ := asUint64(info["l2_tip_height"])
	peerN, _ := asUint64(info["zkproof_v1_peers"])
	if n, ok := asUint64(info["proof_total"]); ok {
		statItems = append(statItems, map[string]interface{}{
			"label": "ZK proofs", "value": fmt.Sprintf("%d", n), "icon": "verified",
		})
	}
	if h, ok := asUint64(info["l2_tip_height"]); ok {
		statItems = append(statItems, map[string]interface{}{
			"label": "L2 tip", "value": fmt.Sprintf("%d", h), "icon": "layers",
		})
	}
	if n, ok := asUint64(info["zkproof_v1_peers"]); ok {
		statItems = append(statItems, map[string]interface{}{
			"label": "Overlay peers", "value": fmt.Sprintf("%d", n), "icon": "hub",
		})
	}
	if gro, _ := info["groth16"].(map[string]interface{}); gro != nil {
		if pairing, _ := gro["pairing_enabled"].(bool); pairing {
			statItems = append(statItems, map[string]interface{}{
				"label": "Groth16", "value": "Ready", "icon": "lock",
			})
		}
	}
	if len(statItems) > 0 {
		widgets = append(widgets, map[string]interface{}{
			"type":  "stats",
			"items": statItems,
		})
	}
	widgets = append(widgets, map[string]interface{}{
		"type":   "metric_chart",
		"title":  "Overlay snapshot",
		"chart":  "bar",
		"labels": []string{"Proofs", "Peers", "L2 tip"},
		"series": []map[string]interface{}{
			{"label": "Value", "color": "#3b82f6", "data": []float64{float64(proofN), float64(peerN), float64(tipH)}},
		},
	})
	if recent, ok := info["recent_proofs"].([]map[string]interface{}); ok && len(recent) > 0 {
		widgets = append(widgets, map[string]interface{}{
			"type":   "proof_list",
			"title":  "Recent proofs",
			"proofs": recent,
		})
	} else if raw, ok := info["recent_proofs"].([]interface{}); ok && len(raw) > 0 {
		proofs := make([]map[string]interface{}, 0, len(raw))
		for _, item := range raw {
			if m, ok := item.(map[string]interface{}); ok {
				proofs = append(proofs, m)
			}
		}
		if len(proofs) > 0 {
			widgets = append(widgets, map[string]interface{}{
				"type":   "proof_list",
				"title":  "Recent proofs",
				"proofs": proofs,
			})
		}
	}
	return widgets
}

func zkl2DashboardTools() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id": "verifyproof", "label": "Verify proof JSON", "method": "verifyproof", "icon": "verified",
			"hint": "Paste a proof object JSON (see USER_GUIDE.md).",
			"fields": []map[string]interface{}{
				{"name": "proof_json", "label": "Proof JSON", "type": "textarea", "placeholder": "{\"proof_hash\":\"...\", ...}"},
			},
		},
		{
			"id": "generateproof", "label": "Generate proof (JSON)", "method": "generateproof", "icon": "auto_awesome", "advanced": true,
			"fields": []map[string]interface{}{
				{"name": "payload_json", "label": "Params JSON", "type": "textarea", "placeholder": "{\"payload\":\"much wow\",\"payload_encoding\":\"text\",\"proof_kind\":\"commitment\"}"},
			},
		},
		{
			"id": "submitproof", "label": "Submit proof", "method": "submitproof", "icon": "upload", "advanced": true,
			"fields": []map[string]interface{}{
				{"name": "proof_json", "label": "Proof JSON", "type": "textarea", "placeholder": "{\"proof_hash\":\"...\", ...}"},
			},
		},
		{
			"id": "proofroot", "label": "Proof root for block", "method": "proofroot", "icon": "account_tree", "advanced": true,
			"fields": []map[string]interface{}{
				{"name": "block_hash", "label": "Block hash", "type": "text", "placeholder": "64-char hex"},
			},
		},
	}
}

func asUint64(v interface{}) (uint64, bool) {
	switch x := v.(type) {
	case uint64:
		return x, true
	case int:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case int64:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	case float64:
		if x < 0 {
			return 0, false
		}
		return uint64(x), true
	default:
		return 0, false
	}
}
