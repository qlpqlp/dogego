// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dogeos

import (
	"fmt"
	"strings"
)

// BuildUI returns host-rendered workspace JSON for the DogeGo Extensions panel.
func BuildUI(snap map[string]interface{}) map[string]interface{} {
	cfg, _ := snap["config"].(Config)
	n, _ := snap["network"].(NetworkProfile)
	last, _ := snap["last_probe"].(ProbeResult)
	metrics, _ := snap["metrics"].(map[string]interface{})
	helpers, _ := snap["helpers"].(map[string]interface{})
	rpcURL, _ := snap["rpc_url"].(string)

	tone := "warn"
	state := "Checking"
	if rpcErr, ok := snap["rpc_error"].(string); ok && rpcErr != "" {
		tone = "warn"
		state = "Unavailable"
	} else if last.OK {
		tone = "ok"
		state = "Live"
	} else if last.Error != "" {
		tone = "danger"
		state = "Down"
	} else if !n.Available && cfg.CustomRPCURL == "" {
		tone = "neutral"
		state = "Soon"
	}

	uptime := fmt.Sprintf("%.0f%%", 0.0)
	avgLat := "—"
	if metrics != nil {
		if v, ok := metrics["uptime_pct_15m"].(float64); ok {
			uptime = fmt.Sprintf("%.0f%%", v)
		}
		if v, ok := metrics["avg_latency_ms"].(int64); ok && v > 0 {
			avgLat = fmt.Sprintf("%dms", v)
		}
	}

	histRows := []map[string]interface{}{}
	if metrics != nil {
		if hist, ok := metrics["history"].([]MetricSample); ok {
			// Newest first, cap for table.
			for i := len(hist) - 1; i >= 0 && len(histRows) < 24; i-- {
				s := hist[i]
				ok := "fail"
				if s.OK {
					ok = "ok"
				}
				histRows = append(histRows, map[string]interface{}{
					"at":     fmt.Sprintf("%d", s.At),
					"ok":     ok,
					"block":  fmt.Sprintf("%d", s.BlockNumber),
					"lat_ms": fmt.Sprintf("%d", s.LatencyMS),
					"gas":    s.GasGwei,
					"err":    s.Error,
				})
			}
		}
	}

	netRows := []map[string]interface{}{}
	if nets, ok := snap["networks"].([]NetworkProfile); ok {
		for _, p := range nets {
			avail := "soon"
			if p.Available {
				avail = "live"
			}
			netRows = append(netRows, map[string]interface{}{
				"id": p.ID, "name": p.Name, "kind": p.Kind,
				"chain_id": fmt.Sprintf("%d", p.ChainID), "status": avail, "notes": p.Notes,
			})
		}
	}

	snippetBody := ""
	if helpers != nil {
		if s, ok := helpers["ethers_v6"].(string); ok {
			snippetBody = s
		}
	}

	return map[string]interface{}{
		"panel_title": "DogeOS",
		"subtitle":    "EVM app layer on Dogecoin · Chikyū testnet live · smart contracts without leaving DogeGo",
		"layout":      "workspace",
		"status_chips": []map[string]interface{}{
			{"id": "rpc", "label": "RPC", "value": state, "tone": tone, "icon": "bolt"},
			{"id": "net", "label": "Network", "value": shortName(n.Name), "tone": "neutral", "icon": "hub"},
			{"id": "block", "label": "Tip", "value": fmt.Sprintf("%d", last.BlockNumber), "tone": "neutral", "icon": "layers"},
			{"id": "lat", "label": "Latency", "value": fmt.Sprintf("%dms", last.LatencyMS), "tone": latTone(last.LatencyMS, last.OK), "icon": "speed"},
			{"id": "up", "label": "Uptime 15m", "value": uptime, "tone": "neutral", "icon": "insights"},
		},
		"nav": []map[string]interface{}{
			{"id": "home", "label": "Home", "icon": "home"},
			{"id": "metrics", "label": "Metrics", "icon": "insights"},
			{"id": "helpers", "label": "Helpers", "icon": "code"},
			{"id": "tools", "label": "Tools", "icon": "construction"},
			{"id": "settings", "label": "Settings", "icon": "tune"},
		},
		"sections": map[string]interface{}{
			"home": map[string]interface{}{
				"title": "Overview",
				"lead":  "DogeOS is an EVM-compatible application layer for Dogecoin. Point wallets and tooling at the RPC below. This extension does not change Dogecoin L1 consensus.",
				"widgets": []map[string]interface{}{
					{"type": "stats", "items": []map[string]interface{}{
						{"label": "Chain ID", "value": fmt.Sprintf("%d", n.ChainID), "icon": "tag"},
						{"label": "Block tip", "value": fmt.Sprintf("%d", last.BlockNumber), "icon": "layers"},
						{"label": "Gas (gwei)", "value": nonempty(last.GasPriceGwei, "—"), "icon": "local_gas_station"},
						{"label": "Avg latency", "value": avgLat, "icon": "speed"},
					}},
					{
						"type": "callout", "tone": toneFromOK(last.OK), "icon": "link",
						"title": "Active RPC",
						"body":  nonempty(rpcURL, "(not configured)"),
					},
					{
						"type": "callout", "tone": "neutral", "icon": "info",
						"title": n.Name,
						"body":  nonempty(n.Notes, "EVM network profile for DogeGo."),
					},
					{
						"type": "links", "title": "Quick links",
						"items": linkItems(n),
					},
					{
						"type": "table", "title": "Networks", "page_size": 6,
						"columns": []map[string]interface{}{
							{"key": "id", "label": "ID"},
							{"key": "name", "label": "Name"},
							{"key": "kind", "label": "Kind"},
							{"key": "chain_id", "label": "Chain ID"},
							{"key": "status", "label": "Status"},
						},
						"rows": netRows,
					},
				},
				"quick_actions": []map[string]interface{}{
					{"id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh"},
					{"id": "probe", "label": "Probe RPC", "method": "probe", "icon": "bolt"},
				},
			},
			"metrics": map[string]interface{}{
				"title": "Live metrics",
				"lead":  "Background probes hit eth_chainId / eth_blockNumber / eth_gasPrice so you can see the network is reachable from this node.",
				"widgets": []map[string]interface{}{
					{"type": "stats", "items": []map[string]interface{}{
						{"label": "Uptime 15m", "value": uptime, "icon": "insights"},
						{"label": "Last latency", "value": fmt.Sprintf("%dms", last.LatencyMS), "icon": "speed"},
						{"label": "Chain match", "value": boolLabel(last.ChainIDMatch), "icon": "verified"},
						{"label": "Client", "value": shortClient(last.ClientVersion), "icon": "memory"},
					}},
					{
						"type": "table", "title": "Probe history", "search": true, "page_size": 10,
						"columns": []map[string]interface{}{
							{"key": "at", "label": "Unix"},
							{"key": "ok", "label": "OK"},
							{"key": "block", "label": "Block"},
							{"key": "lat_ms", "label": "ms"},
							{"key": "gas", "label": "gwei"},
							{"key": "err", "label": "Error"},
						},
						"rows": histRows,
					},
				},
				"quick_actions": []map[string]interface{}{
					{"id": "probe", "label": "Probe now", "method": "probe", "icon": "bolt"},
					{"id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh"},
				},
			},
			"helpers": map[string]interface{}{
				"title": "Developer helpers",
				"lead":  "Copy MetaMask add-network JSON, Hardhat/Foundry/ethers/viem snippets, and cast/curl one-liners for the active profile.",
				"widgets": []map[string]interface{}{
					{
						"type": "callout", "tone": "neutral", "icon": "code",
						"title": "ethers v6",
						"body":  snippetBody,
					},
					{
						"type": "callout", "tone": "neutral", "icon": "terminal",
						"title": "curl tip block",
						"body":  strHelper(helpers, "curl_block"),
					},
					{
						"type": "callout", "tone": "neutral", "icon": "terminal",
						"title": "cast",
						"body":  strHelper(helpers, "cast_block"),
					},
					{
						"type": "callout", "tone": "neutral", "icon": "settings",
						"title": "Hardhat network",
						"body":  strHelper(helpers, "hardhat_network"),
					},
					{
						"type": "callout", "tone": "neutral", "icon": "settings",
						"title": "Foundry rpc_endpoints",
						"body":  strHelper(helpers, "foundry_toml"),
					},
				},
				"tools": []map[string]interface{}{
					{
						"id": "helpers", "label": "Get all snippets (JSON)", "method": "helpers", "icon": "code",
						"params_as": "object", "fields": []map[string]interface{}{},
					},
					{
						"id": "metamask", "label": "MetaMask addEthereumChain params", "method": "helpers", "icon": "account_balance_wallet",
						"params_as": "object", "fields": []map[string]interface{}{},
					},
				},
				"quick_actions": []map[string]interface{}{
					{"id": "helpers", "label": "Load helpers", "method": "helpers", "icon": "code"},
				},
			},
			"tools": map[string]interface{}{
				"title": "RPC tools",
				"lead":  "Read-only EVM helpers against the configured DogeOS RPC. For writes, use your wallet or Hardhat/Foundry against the same endpoint.",
				"tools": []map[string]interface{}{
					{
						"id": "probe", "label": "Probe RPC", "method": "probe", "icon": "bolt",
						"params_as": "object", "fields": []map[string]interface{}{},
					},
					{
						"id": "balance", "label": "Get DOGE balance", "method": "getbalance", "icon": "account_balance_wallet",
						"params_as": "object",
						"fields": []map[string]interface{}{
							{"name": "address", "label": "EVM address (0x…)", "type": "text", "placeholder": "0x…"},
						},
					},
					{
						"id": "code", "label": "Is contract?", "method": "getcode", "icon": "memory",
						"params_as": "object",
						"fields": []map[string]interface{}{
							{"name": "address", "label": "EVM address (0x…)", "type": "text"},
						},
					},
					{
						"id": "receipt", "label": "Transaction receipt", "method": "getreceipt", "icon": "receipt",
						"params_as": "object",
						"fields": []map[string]interface{}{
							{"name": "tx_hash", "label": "Tx hash (0x…)", "type": "text"},
						},
					},
					{
						"id": "block", "label": "Get block", "method": "getblock", "icon": "layers",
						"params_as": "object",
						"fields": []map[string]interface{}{
							{"name": "number", "label": "Block (latest or number)", "type": "text", "placeholder": "latest"},
						},
					},
					{
						"id": "rpc", "label": "Raw JSON-RPC call", "method": "rpccall", "icon": "terminal",
						"params_as": "object",
						"fields": []map[string]interface{}{
							{"name": "method", "label": "Method", "type": "text", "placeholder": "eth_blockNumber"},
							{"name": "params_json", "label": "Params JSON array", "type": "textarea", "placeholder": "[]"},
						},
					},
				},
			},
			"settings": map[string]interface{}{
				"title": "Settings",
				"lead":  "Chikyū testnet is the default. Switch to mainnet when DogeOS publishes it, or override RPC for a private endpoint.",
				"tools": []map[string]interface{}{
					{
						"id": "setconfig", "label": "Save settings", "method": "setconfig", "icon": "save",
						"params_as": "object",
						"fields": []map[string]interface{}{
							{
								"name": "network_id", "label": "Network profile", "type": "select",
								"options": []map[string]interface{}{
									{"value": NetworkChikyuTestnet, "label": "Chikyū Testnet (live)"},
									{"value": NetworkMainnetSoon, "label": "Mainnet (when available)"},
								},
								"value": cfg.NetworkID,
							},
							{"name": "custom_rpc_url", "label": "Custom RPC URL (optional)", "type": "text", "placeholder": "https://rpc.testnet.dogeos.com/", "value": cfg.CustomRPCURL},
							{"name": "poll_seconds", "label": "Metrics poll interval (seconds)", "type": "text", "value": fmt.Sprintf("%d", cfg.PollSeconds)},
							{"name": "metrics_enabled", "label": "Background metrics", "type": "checkbox", "value": cfg.MetricsEnabled},
						},
					},
					{
						"id": "getconfig", "label": "Show config", "method": "getconfig", "icon": "settings",
						"params_as": "object", "fields": []map[string]interface{}{},
					},
				},
			},
		},
	}
}

func shortName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	if strings.Contains(s, "Chikyū") || strings.Contains(strings.ToLower(s), "chikyu") {
		return "Chikyū"
	}
	if strings.Contains(strings.ToLower(s), "mainnet") {
		return "Mainnet"
	}
	if len(s) > 18 {
		return s[:18] + "…"
	}
	return s
}

func shortClient(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	if len(s) > 22 {
		return s[:22] + "…"
	}
	return s
}

func latTone(ms int64, ok bool) string {
	if !ok {
		return "danger"
	}
	if ms <= 0 {
		return "neutral"
	}
	if ms < 400 {
		return "ok"
	}
	if ms < 1500 {
		return "warn"
	}
	return "danger"
}

func toneFromOK(ok bool) string {
	if ok {
		return "ok"
	}
	return "warn"
}

func boolLabel(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func nonempty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func strHelper(h map[string]interface{}, key string) string {
	if h == nil {
		return ""
	}
	if s, ok := h[key].(string); ok {
		return s
	}
	return ""
}

func linkItems(n NetworkProfile) []map[string]interface{} {
	items := []map[string]interface{}{}
	add := func(label, url, icon string) {
		if strings.TrimSpace(url) == "" {
			return
		}
		items = append(items, map[string]interface{}{"label": label, "url": url, "icon": icon})
	}
	add("Docs", n.DocsURL, "menu_book")
	add("Explorer", n.ExplorerURL, "travel_explore")
	add("Faucet", n.FaucetURL, "water_drop")
	add("Bridge guide", n.BridgeURL, "swap_horiz")
	add("Wallet setup", n.PortalURL, "account_balance_wallet")
	return items
}
