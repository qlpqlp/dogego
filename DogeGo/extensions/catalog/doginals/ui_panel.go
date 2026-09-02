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
		"panel_title": "Doginals L2",
		"subtitle":    "Index Doginals / DRC-20 / Ordinals on L1, sync L2 assets with peers, mint on-chain or off-L1.",
		"summary":     "Wizard · Doginals wallet API · doginals-v1 P2P",
		"status_chips": chips,
		"layout":       "workspace",
		"nav": []map[string]interface{}{
			{"id": "setup", "label": "1. Setup", "icon": "tune"},
			{"id": "sync", "label": "2. Sync", "icon": "cloud_sync"},
			{"id": "create", "label": "3. Create", "icon": "auto_awesome"},
			{"id": "api", "label": "4. Wallet API", "icon": "api"},
			{"id": "browse", "label": "Browse", "icon": "grid_view"},
		},
		"sections": map[string]interface{}{
			"setup": map[string]interface{}{
				"title": "Step 1 — Setup",
				"lead":  "Enable wallet RPC if you plan to mint on Dogecoin L1. Settings persist under data/ across upgrades.",
				"wizards": []map[string]interface{}{
					{
						"id": "setup_wizard", "title": "Quick setup", "method": "setconfig", "icon": "check_circle",
						"finish_label": "Save settings",
						"steps": []map[string]interface{}{
							{
								"id": "wallet", "title": "Wallet",
								"hint": "Unlock the dashboard wallet before L1 mint/deploy broadcast.",
								"fields": []map[string]interface{}{
									{"name": "wallet_rpc_enabled", "label": "Enable wallet RPC", "type": "switch", "default": boolStr(walletOn),
										"hint": "Required for on-chain DRC-20 inscribe."},
									{"name": "auto_broadcast", "label": "Auto-broadcast after sign", "type": "switch", "default": boolStr(autoBC)},
									{"name": "preferred_address", "label": "Preferred address", "type": "text", "placeholder": "D…", "default": prefAddr},
								},
							},
						},
					},
				},
				"quick_actions": []map[string]interface{}{
					{"id": "getconfig", "label": "Reload settings", "method": "getconfig", "icon": "refresh"},
					{"id": "exportbackup", "label": "Export backup", "method": "exportbackup", "icon": "backup"},
				},
			},
			"sync": map[string]interface{}{
				"title": "Step 2 — Sync index",
				"lead":  "L1 index runs locally as blocks connect. L2 assets gossip via doginals-v1 among DogeGo peers (like block sync for metadata).",
				"widgets": []map[string]interface{}{
					{
						"type":    "progress",
						"label":   "L1 index through chain tip",
						"percent": syncPct,
						"value":   fmt.Sprintf("%d / %d (lag %d)", idx, tip, lag),
					},
					{
						"type":  "callout",
						"tone":  "neutral",
						"icon":  "hub",
						"title": "Decentralized L2 overlay",
						"body":  "Protocol doginals-v1 · commands dinv / getdasset / dasset. Background sync every 60s. Not Dogecoin consensus — experimental second layer for metadata and off-L1 balances.",
					},
				},
				"tools": []map[string]interface{}{
					{
						"id": "indexrange", "label": "Backfill L1 heights", "method": "indexrange", "icon": "manage_search",
						"hint": "Scan up to 5000 blocks per run to catch historical doginals/DRC-20.",
						"fields": []map[string]interface{}{
							{"name": "from_height", "label": "From height", "type": "number", "placeholder": "0"},
							{"name": "to_height", "label": "To height", "type": "number", "placeholder": "1000"},
						},
					},
				},
				"quick_actions": []map[string]interface{}{
					{"id": "sync", "label": "Sync status", "method": "syncstatus", "icon": "cloud_sync"},
					{"id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh"},
				},
			},
			"create": map[string]interface{}{
				"title": "Step 3 — Create",
				"lead":  "Mint DRC-20 on Dogecoin (L1) or experimental off-L1 balances (L2) without an on-chain tx.",
				"wizards": []map[string]interface{}{
					{
						"id": "drc20_wizard", "title": "DRC-20 on L1", "method": "inscribe", "icon": "currency_bitcoin",
						"finish_label": "Build / broadcast",
						"steps": []map[string]interface{}{
							{
								"id": "op", "title": "Operation",
								"fields": []map[string]interface{}{
									{"name": "op", "label": "Operation", "type": "select", "default": "mint",
										"options": []map[string]string{
											{"value": "deploy", "label": "Deploy"},
											{"value": "mint", "label": "Mint"},
											{"value": "transfer", "label": "Transfer"},
										}},
								},
							},
							{
								"id": "params", "title": "Token",
								"fields": []map[string]interface{}{
									{"name": "tick", "label": "Ticker", "type": "text", "placeholder": "woof"},
									{"name": "amt", "label": "Amount", "type": "text", "placeholder": "1000"},
									{"name": "max", "label": "Max (deploy)", "type": "text"},
									{"name": "lim", "label": "Limit (deploy)", "type": "text"},
								},
							},
							{
								"id": "broadcast", "title": "Broadcast",
								"fields": []map[string]interface{}{
									{"name": "broadcast", "label": "Broadcast to Dogecoin", "type": "switch", "default": "false"},
								},
							},
						},
					},
					{
						"id": "l2_mint_wizard", "title": "Mint on L2 (off-chain)", "method": "mintl2", "icon": "layers",
						"finish_label": "Credit L2 balance",
						"steps": []map[string]interface{}{
							{
								"id": "who", "title": "Recipient",
								"hint": "Experimental: credits local/P2P-synced ledger, not Dogecoin consensus.",
								"fields": []map[string]interface{}{
									{"name": "address", "label": "Dogecoin address", "type": "text", "placeholder": "D…"},
									{"name": "tick", "label": "Ticker", "type": "text", "placeholder": "woof"},
									{"name": "amount", "label": "Amount", "type": "text", "placeholder": "1000"},
								},
							},
							{
								"id": "meta", "title": "Asset metadata (optional)",
								"fields": []map[string]interface{}{
									{"name": "name", "label": "Name", "type": "text"},
									{"name": "uri", "label": "URI", "type": "text", "placeholder": "ipfs://…"},
								},
							},
						},
					},
				},
			},
			"api": map[string]interface{}{
				"title": "Step 4 — Wallet API",
				"lead":  "Doginals wallet read routes for mobile wallets and websites. Enable this extension on your node.",
				"widgets": []map[string]interface{}{
					{
						"type":  "callout",
						"tone":  "ok",
						"icon":  "api",
						"title": "Public read API (CORS *)",
						"body":  "GET /api/ext/dogego.doginals/v1/status · /tokens · /address/{addr} · /address/{addr}/history?tick= · /txid/{txid}. POST /mint/l2 and /inscribe require dashboard unlock. Host only proxies /api/ext/{id}/… — routes live in this extension.",
					},
					{
						"type": "item_list",
						"title": "Example routes",
						"items": []map[string]interface{}{
							{"title": "Status", "meta": "GET /api/ext/dogego.doginals/v1/status", "id": "status"},
							{"title": "Address balances", "meta": "GET /api/ext/dogego.doginals/v1/address/D…", "id": "address"},
							{"title": "Token list", "meta": "GET /api/ext/dogego.doginals/v1/tokens", "id": "tokens"},
							{"title": "Events by txid", "meta": "GET /api/ext/dogego.doginals/v1/txid/{txid}", "id": "txid"},
						},
					},
				},
				"quick_actions": []map[string]interface{}{
					{"id": "apistatus", "label": "API manifest", "method": "apistatus", "icon": "description"},
				},
			},
			"browse": map[string]interface{}{
				"title": "Browse index",
				"lead":  "Tokens, inscriptions, and L2 assets already indexed on this node.",
				"widgets": append([]map[string]interface{}{
					{
						"type": "stats",
						"items": []map[string]interface{}{
							{"label": "Tokens", "value": fmt.Sprintf("%d", tokN), "icon": "token"},
							{"label": "Inscriptions", "value": fmt.Sprintf("%d", insN), "icon": "receipt_long"},
							{"label": "L2 assets", "value": fmt.Sprintf("%d", assetN), "icon": "collections"},
						},
					},
				}, tokenTableWidget(info)...),
				"quick_actions": []map[string]interface{}{
					{"id": "list_ins", "label": "Recent inscriptions", "method": "listinscriptions", "icon": "list", "params": []interface{}{float64(20)}},
					{"id": "list_assets", "label": "L2 gallery", "method": "listassets", "icon": "grid_view", "params": []interface{}{float64(20)}},
					{"id": "list_tok", "label": "Refresh tokens", "method": "listtokens", "icon": "refresh", "params": []interface{}{float64(40)}},
				},
				"tools": []map[string]interface{}{
					{
						"id": "gettoken", "label": "Lookup ticker", "method": "gettoken", "icon": "search",
						"fields": []map[string]interface{}{
							{"name": "tick", "label": "Ticker", "type": "text"},
						},
					},
					{
						"id": "getaddress", "label": "Address balances", "method": "getaddress", "icon": "account_balance_wallet",
						"fields": []map[string]interface{}{
							{"name": "address", "label": "Address", "type": "text", "placeholder": "D…"},
						},
					},
					{
						"id": "putasset", "label": "Create L2 asset", "method": "putasset", "icon": "add_photo_alternate",
						"fields": []map[string]interface{}{
							{"name": "kind", "label": "Kind", "type": "select", "default": "nft",
								"options": []map[string]string{
									{"value": "nft", "label": "NFT"},
									{"value": "token", "label": "Token"},
									{"value": "image", "label": "Image"},
									{"value": "collection", "label": "Collection"},
								}},
							{"name": "name", "label": "Name", "type": "text"},
							{"name": "uri", "label": "URI", "type": "text"},
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
