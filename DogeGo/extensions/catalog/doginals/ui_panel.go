// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import "fmt"

func buildUIPanel(info map[string]interface{}) map[string]interface{} {
	insN, _ := asInt(info["inscriptions"])
	assetN, _ := asInt(info["l2_assets"])
	tokN, _ := asInt(info["tokens"])
	idx, _ := asInt64(info["index_height"])
	tip, _ := asInt64(info["chain_tip"])
	cfg, _ := info["config"].(map[string]interface{})
	if cfg == nil {
		if c, ok := info["config"].(ExtConfig); ok {
			cfg = map[string]interface{}{
				"wallet_rpc_enabled": c.WalletRPCEnabled,
				"auto_broadcast":     c.AutoBroadcast,
				"preferred_address":  c.PreferredAddress,
			}
		} else {
			cfg = map[string]interface{}{}
		}
	}
	walletOn, _ := cfg["wallet_rpc_enabled"].(bool)
	autoBC, _ := cfg["auto_broadcast"].(bool)
	prefAddr, _ := cfg["preferred_address"].(string)

	chips := []map[string]interface{}{
		{"id": "proto", "label": "Overlay", "value": ProtocolID, "tone": "neutral", "icon": "hub"},
		{"id": "tokens", "label": "DRC-20 tokens", "value": fmt.Sprintf("%d", tokN), "tone": "ok", "icon": "token"},
		{"id": "l1", "label": "Inscriptions", "value": fmt.Sprintf("%d", insN), "tone": "ok", "icon": "receipt_long"},
		{"id": "l2", "label": "L2 assets", "value": fmt.Sprintf("%d", assetN), "tone": "ok", "icon": "collections"},
	}
	lagTone := "ok"
	lag := int64(0)
	if tip >= 0 && idx >= 0 {
		lag = tip - idx
		if lag > 100 {
			lagTone = "warn"
		}
	}
	chips = append(chips, map[string]interface{}{
		"id": "sync", "label": "Index tip", "value": fmt.Sprintf("%d / %d", idx, tip), "tone": lagTone, "icon": "sync",
	})
	walletTone := "warn"
	walletVal := "Off"
	if walletOn {
		walletTone = "ok"
		walletVal = "On"
	}
	chips = append(chips, map[string]interface{}{
		"id": "wallet", "label": "Wallet RPC", "value": walletVal, "tone": walletTone, "icon": "account_balance_wallet",
	})

	syncPct := 0.0
	if tip > 0 && idx >= 0 {
		syncPct = float64(idx) / float64(tip) * 100
		if syncPct > 100 {
			syncPct = 100
		}
	}

	return map[string]interface{}{
		"panel_title": "Doginals & DRC-20",
		"subtitle":    "Index on-chain inscriptions and tokens. Mint DRC-20 via authenticated wallet RPC. Sync L2 assets with peers.",
		"summary":     "Experimental L2 · L1 observe + optional wallet mint · modern workspace below",
		"status_chips": chips,
		"layout":       "workspace",
		"nav": []map[string]interface{}{
			{"id": "home", "label": "Home", "icon": "home"},
			{"id": "tokens", "label": "Tokens", "icon": "token"},
			{"id": "mint", "label": "Mint / Deploy", "icon": "auto_awesome"},
			{"id": "inscriptions", "label": "Inscriptions", "icon": "receipt_long"},
			{"id": "assets", "label": "L2 Assets", "icon": "collections"},
			{"id": "index", "label": "Index", "icon": "manage_search"},
			{"id": "settings", "label": "Settings", "icon": "tune"},
		},
		"sections": map[string]interface{}{
			"home": map[string]interface{}{
				"title": "Overview",
				"lead":  "Status and metrics. Use the menu for tokens, minting, and settings.",
				"widgets": append([]map[string]interface{}{
					{
						"type": "stats",
						"items": []map[string]interface{}{
							{"label": "Tokens", "value": fmt.Sprintf("%d", tokN), "icon": "token"},
							{"label": "Inscriptions", "value": fmt.Sprintf("%d", insN), "icon": "receipt_long"},
							{"label": "L2 assets", "value": fmt.Sprintf("%d", assetN), "icon": "collections"},
							{"label": "Index lag", "value": fmt.Sprintf("%d", lag), "icon": "speed"},
						},
					},
					{
						"type":    "progress",
						"label":   "Index sync",
						"percent": syncPct,
						"value":   fmt.Sprintf("%d / %d", idx, tip),
					},
					overviewChart(tokN, insN, assetN, idx, tip),
				}, tokenListWidget(info)...),
				"quick_actions": []map[string]interface{}{
					{"id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh"},
					{"id": "list_tok", "label": "List tokens", "method": "listtokens", "icon": "token", "params": []interface{}{float64(20)}},
					{"id": "sync", "label": "Sync status", "method": "syncstatus", "icon": "cloud_sync"},
				},
			},
			"tokens": map[string]interface{}{
				"title": "DRC-20 token index",
				"lead":  "Search and browse indexed deploy/mint/transfer events.",
				"quick_actions": []map[string]interface{}{
					{"id": "list_tok", "label": "Refresh tokens", "method": "listtokens", "icon": "refresh", "params": []interface{}{float64(40)}},
				},
				"widgets": tokenTableWidget(info),
				"tools": []map[string]interface{}{
					{
						"id": "gettoken", "label": "Get token", "method": "gettoken", "icon": "search",
						"fields": []map[string]interface{}{
							{"name": "tick", "label": "Ticker", "type": "text", "placeholder": "woof"},
						},
					},
					{
						"id": "listbytick", "label": "Events for ticker", "method": "listbytick", "icon": "history", "advanced": true,
						"fields": []map[string]interface{}{
							{"name": "tick", "label": "Ticker", "type": "text", "placeholder": "woof"},
							{"name": "limit", "label": "Limit", "type": "number", "default": "40"},
						},
					},
				},
			},
			"mint": map[string]interface{}{
				"title": "Deploy / mint / transfer",
				"lead":  "Guided form for DRC-20 JSON. Broadcast needs wallet RPC enabled in Settings and an unlocked dashboard wallet.",
				"wizards": []map[string]interface{}{
					{
						"id": "drc20_wizard", "title": "DRC-20 wizard", "method": "inscribe", "icon": "auto_awesome",
						"finish_label": "Build / broadcast",
						"steps": []map[string]interface{}{
							{
								"id": "op", "title": "Operation",
								"hint": "deploy creates a new tick; mint increases supply toward max; transfer records a move.",
								"fields": []map[string]interface{}{
									{"name": "op", "label": "Operation", "type": "select", "default": "mint",
										"options": []map[string]string{
											{"value": "deploy", "label": "Deploy", "icon": "rocket_launch"},
											{"value": "mint", "label": "Mint", "icon": "auto_awesome"},
											{"value": "transfer", "label": "Transfer", "icon": "swap_horiz"},
										}},
								},
							},
							{
								"id": "params", "title": "Token parameters",
								"fields": []map[string]interface{}{
									{"name": "tick", "label": "Ticker", "type": "text", "placeholder": "woof"},
									{"name": "amt", "label": "Amount (mint/transfer)", "type": "text", "placeholder": "1000"},
									{"name": "max", "label": "Max supply (deploy)", "type": "text", "placeholder": "21000000"},
									{"name": "lim", "label": "Mint limit (deploy, optional)", "type": "text", "placeholder": "1000"},
								},
							},
							{
								"id": "broadcast", "title": "Broadcast",
								"hint": "Leave off to preview and fund only.",
								"fields": []map[string]interface{}{
									{"name": "broadcast", "label": "Broadcast now", "type": "switch", "default": "false",
										"hint": "Requires wallet RPC enabled in Settings."},
								},
							},
						},
					},
				},
				"tools": []map[string]interface{}{
					{
						"id": "preview", "label": "Preview JSON only", "method": "previewinscription", "icon": "preview", "advanced": true,
						"fields": []map[string]interface{}{
							{"name": "op", "label": "op", "type": "text", "default": "mint"},
							{"name": "tick", "label": "tick", "type": "text", "placeholder": "woof"},
							{"name": "amt", "label": "amt", "type": "text", "placeholder": "100"},
							{"name": "max", "label": "max", "type": "text"},
							{"name": "lim", "label": "lim", "type": "text"},
						},
					},
				},
			},
			"inscriptions": map[string]interface{}{
				"title": "L1 inscriptions",
				"lead":  "Recent OP_RETURN / data-carrier observations.",
				"quick_actions": []map[string]interface{}{
					{"id": "list_ins", "label": "Recent", "method": "listinscriptions", "icon": "list", "params": []interface{}{float64(20)}},
				},
				"tools": []map[string]interface{}{
					{
						"id": "getinscription", "label": "Get inscription", "method": "getinscription", "icon": "search",
						"fields": []map[string]interface{}{
							{"name": "id", "label": "Inscription id", "type": "text", "placeholder": "txidivout@height"},
						},
					},
				},
			},
			"assets": map[string]interface{}{
				"title": "Off-L1 L2 assets",
				"lead":  "NFT / token / image metadata synced via doginals-v1 (not L1).",
				"quick_actions": []map[string]interface{}{
					{"id": "list_assets", "label": "Gallery", "method": "listassets", "icon": "grid_view", "params": []interface{}{float64(20)}},
				},
				"tools": []map[string]interface{}{
					{
						"id": "putasset", "label": "Create L2 asset", "method": "putasset", "icon": "add_photo_alternate",
						"hint": "Off-chain metadata only. Survives upgrades in data/.",
						"fields": []map[string]interface{}{
							{"name": "kind", "label": "Kind", "type": "select", "default": "nft",
								"options": []map[string]string{
									{"value": "nft", "label": "NFT", "icon": "image"},
									{"value": "token", "label": "Token", "icon": "token"},
									{"value": "image", "label": "Image", "icon": "photo"},
									{"value": "collection", "label": "Collection", "icon": "collections"},
								}},
							{"name": "name", "label": "Name", "type": "text", "placeholder": "Much Wow #1"},
							{"name": "description", "label": "Description", "type": "textarea"},
							{"name": "content_type", "label": "Content type", "type": "text", "default": "image/png"},
							{"name": "uri", "label": "URI / URL", "type": "text", "placeholder": "ipfs://…"},
							{"name": "l1_inscription_id", "label": "Link L1 id", "type": "text"},
						},
					},
					{
						"id": "getasset", "label": "Get L2 asset", "method": "getasset", "icon": "inventory_2", "advanced": true,
						"fields": []map[string]interface{}{
							{"name": "id", "label": "Asset id", "type": "text"},
						},
					},
				},
			},
			"index": map[string]interface{}{
				"title": "L1 index",
				"lead":  "Backfill heights into the local index (max 5000 per run).",
				"widgets": []map[string]interface{}{
					{
						"type":    "progress",
						"label":   "Indexed through tip",
						"percent": syncPct,
						"value":   fmt.Sprintf("%d / %d", idx, tip),
					},
				},
				"tools": []map[string]interface{}{
					{
						"id": "indexrange", "label": "Scan L1 range", "method": "indexrange", "icon": "manage_search",
						"fields": []map[string]interface{}{
							{"name": "from_height", "label": "From height", "type": "number", "placeholder": "0"},
							{"name": "to_height", "label": "To height", "type": "number", "placeholder": "1000"},
						},
					},
				},
			},
			"settings": map[string]interface{}{
				"title": "Extension settings",
				"lead":  "Preferences live in data/ and survive zip upgrades. Export a backup before moving nodes.",
				"widgets": []map[string]interface{}{
					{
						"type":  "callout",
						"tone":  "ok",
						"icon":  "folder_special",
						"title": "Upgrade-safe storage",
						"body":  "Install/Update keeps extensions/dogego.doginals/data/ (index DB + settings). Backups are written under data/backups/.",
					},
				},
				"tools": []map[string]interface{}{
					{
						"id": "setconfig", "label": "Save settings", "method": "setconfig", "icon": "save", "run_label": "Save",
						"hint": "Modern toggles below. Wallet RPC is required for mint/deploy broadcast.",
						"fields": []map[string]interface{}{
							{"name": "wallet_rpc_enabled", "label": "Enable wallet RPC for this extension", "type": "switch", "default": boolStr(walletOn),
								"hint": "Uses authenticated DogeGo RPC only; keys never leave the node wallet."},
							{"name": "auto_broadcast", "label": "Auto-broadcast after fund/sign", "type": "switch", "default": boolStr(autoBC)},
							{"name": "preferred_address", "label": "Preferred address (optional)", "type": "text", "placeholder": "D…", "default": prefAddr},
						},
					},
					{
						"id": "getconfig", "label": "Reload settings", "method": "getconfig", "icon": "download", "advanced": true,
					},
					{
						"id": "exportbackup", "label": "Export backup", "method": "exportbackup", "icon": "backup",
						"hint": "Copies settings JSON into data/backups/ and returns the payload.",
					},
					{
						"id": "importbackup", "label": "Restore backup", "method": "importbackup", "icon": "restore", "advanced": true,
						"fields": []map[string]interface{}{
							{"name": "backup_json", "label": "Backup JSON", "type": "textarea", "placeholder": "{\"config\":{...}}"},
						},
					},
				},
			},
		},
	}
}

func overviewChart(tokN, insN, assetN int, idx, tip int64) map[string]interface{} {
	return map[string]interface{}{
		"type":   "metric_chart",
		"title":  "Activity snapshot",
		"lead":   "Current indexed counts (refreshes with the panel).",
		"chart":  "bar",
		"labels": []string{"Tokens", "Inscriptions", "L2 assets", "Index k", "Tip k"},
		"series": []map[string]interface{}{
			{
				"label": "Count",
				"color": "#c2a633",
				"data": []float64{
					float64(tokN),
					float64(insN),
					float64(assetN),
					float64(idx) / 1000.0,
					float64(tip) / 1000.0,
				},
			},
		},
	}
}

func tokenListWidget(info map[string]interface{}) []map[string]interface{} {
	rows := tokenRows(info)
	if len(rows) == 0 {
		return nil
	}
	items := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]interface{}{
			"title": r["tick"],
			"meta":  fmt.Sprintf("mints %v · transfers %v · max %v", r["mints"], r["transfers"], r["max"]),
			"id":    r["tick"],
		})
	}
	return []map[string]interface{}{{
		"type":      "item_list",
		"title":     "Recent tokens",
		"page_size": 8,
		"items":     items,
	}}
}

func tokenTableWidget(info map[string]interface{}) []map[string]interface{} {
	rows := tokenRows(info)
	return []map[string]interface{}{{
		"type":               "table",
		"title":              "Indexed tokens",
		"search":             true,
		"search_placeholder": "Search ticker…",
		"page_size":          15,
		"columns": []map[string]interface{}{
			{"key": "tick", "label": "Ticker"},
			{"key": "mints", "label": "Mints"},
			{"key": "transfers", "label": "Transfers"},
			{"key": "max", "label": "Max"},
		},
		"rows": rows,
		"load_more": map[string]interface{}{
			"method":            "listtokens",
			"params":            []interface{}{float64(80)},
			"limit_param_index": 0,
			"map_rows":          "tokens",
		},
	}}
}

func tokenRows(info map[string]interface{}) []map[string]interface{} {
	raw, ok := info["recent_tokens"]
	if !ok {
		return nil
	}
	var rows []map[string]interface{}
	switch v := raw.(type) {
	case []TokenSummary:
		for _, t := range v {
			rows = append(rows, map[string]interface{}{
				"tick": t.Tick, "mints": t.MintCount, "transfers": t.TransferCount, "max": t.Max,
			})
		}
	case []interface{}:
		for _, item := range v {
			m, _ := item.(map[string]interface{})
			if m == nil {
				continue
			}
			tick, _ := m["tick"].(string)
			rows = append(rows, map[string]interface{}{
				"tick":      tick,
				"mints":     m["mint_count"],
				"transfers": m["transfer_count"],
				"max":       m["max"],
			})
		}
	}
	return rows
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func asInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

func asInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	default:
		return 0, false
	}
}
