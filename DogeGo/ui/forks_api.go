// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package ui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"dogego/analytics"
	"dogego/pow"
)

// BuildForksStatus aggregates peer tip divergence, soft-fork deployments, and recent reorgs
// for the Analytics → Forks dashboard (mainnet and reboot testnet share the same machinery).
func BuildForksStatus(cfg StartConfig) map[string]any {
	out := map[string]any{
		"network":      cfg.Network,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"glossary": []map[string]string{
			{"id": "reorg", "title": "Chain reorg (fork competition)", "body": "Two valid tip histories share an ancestor; the node follows the heavier chain and may truncate the lighter tip. This is normal PoW competition, not a protocol hard fork."},
			{"id": "soft", "title": "Soft fork", "body": "Tightened rules that old nodes still see as valid (BIP9/buried deployments like CSV). Signaling and activation appear under Deployments."},
			{"id": "hard", "title": "Hard fork / rule split", "body": "Incompatible consensus rules. Surfaces as lasting tip-hash divergence and repeated header/block rejects. DogeGo does not auto-switch networks; check build/network magic and peer agreement."},
			{"id": "diverged", "title": "Peer tip diverged", "body": "Peer reports the same height as this node (or close) but a different tip hash: competing tip or stale view."},
		},
	}

	local := map[string]any{"height": int64(-1), "hash": "", "chain": cfg.Network}
	if cfg.ActiveJournal() != nil {
		if tip, err := cfg.ActiveJournal().TipHeight(); err == nil && tip >= 0 {
			local["height"] = tip
			if h80, err := cfg.ActiveJournal().ReadHeaderAt(tip); err == nil {
				local["hash"] = pow.BlockHashHex(h80)
			}
		}
	}
	out["local_tip"] = local
	localH, _ := local["height"].(int64)
	localHash, _ := local["hash"].(string)
	localHash = strings.ToLower(strings.TrimSpace(localHash))

	peers := []map[string]any{}
	var diverged, ahead, behind, aligned int
	if cfg.RPCInvoke != nil {
		res := cfg.RPCInvoke("getpeerinfo", nil)
		if arr := rpcResultArray(res); len(arr) > 0 {
			for _, item := range arr {
				row, _ := item.(map[string]any)
				if row == nil {
					continue
				}
				ph := asInt64Def(row["synced_headers"], asInt64Def(row["startingheight"], -1))
				phash := strings.ToLower(strings.TrimSpace(fmt.Sprint(row["dogego_best_header_hash"])))
				if phash == "<nil>" || phash == "" {
					phash = ""
				}
				status, detail := classifyPeerTip(localH, localHash, ph, phash)
				switch status {
				case "diverged":
					diverged++
				case "ahead":
					ahead++
				case "behind":
					behind++
				case "aligned":
					aligned++
				}
				peers = append(peers, map[string]any{
					"addr":            fmt.Sprint(row["addr"]),
					"subver":          fmt.Sprint(row["subver"]),
					"inbound":         row["inbound"],
					"role":            fmt.Sprint(row["dogego_role"]),
					"synced_headers":  ph,
					"startingheight":  asInt64Def(row["startingheight"], -1),
					"tip_hash":        phash,
					"tip_updated":     row["dogego_tip_updated"],
					"delta_height":    ph - localH,
					"status":          status,
					"status_detail":   detail,
					"dgr_tunnel":      row["dogego_dgr_tunnel"] == true,
				})
			}
		}
	}
	out["peer_tips"] = peers
	out["peer_summary"] = map[string]any{
		"total":    len(peers),
		"aligned":  aligned,
		"behind":   behind,
		"ahead":    ahead,
		"diverged": diverged,
	}

	deployments := []map[string]any{}
	if cfg.RPCInvoke != nil {
		depRes := cfg.RPCInvoke("getdeploymentinfo", nil)
		if dep := rpcResultMap(depRes); dep != nil {
			out["deployment_hash"] = dep["hash"]
			out["deployment_height"] = dep["height"]
			if m, ok := dep["deployments"].(map[string]any); ok {
				for name, raw := range m {
					dm, _ := raw.(map[string]any)
					if dm == nil {
						continue
					}
					row := map[string]any{
						"name":   name,
						"type":   dm["type"],
						"active": dm["active"],
						"height": dm["height"],
					}
					if bip9, ok := dm["bip9"].(map[string]any); ok {
						row["bip9_status"] = bip9["status"]
						row["bit"] = bip9["bit"]
						row["start_time"] = bip9["start_time"]
						row["timeout"] = bip9["timeout"]
						row["since"] = bip9["since"]
						if st, ok := bip9["statistics"].(map[string]any); ok {
							row["period"] = st["period"]
							row["threshold"] = st["threshold"]
							row["elapsed"] = st["elapsed"]
							row["count"] = st["count"]
							row["possible"] = st["possible"]
						}
					}
					if buried, ok := dm["height"]; ok && dm["type"] == "buried" {
						row["activation_height"] = buried
					}
					deployments = append(deployments, row)
				}
			}
		}
		infoRes := cfg.RPCInvoke("getblockchaininfo", nil)
		if info := rpcResultMap(infoRes); info != nil {
			out["softforks"] = info["softforks"]
			out["bip9_softforks"] = info["bip9_softforks"]
			out["verificationprogress"] = info["verificationprogress"]
			out["initialblockdownload"] = info["initialblockdownload"]
		}
	}
	out["deployments"] = deployments

	dbPath := filepath.Join(cfg.ChainDataDir, "dogego_analytics.db")
	var detail *analytics.SideDetail
	var err error
	if cfg.AnalyticsRead != nil {
		detail, err = cfg.AnalyticsRead()
	} else {
		detail, err = analytics.ReadSideDetail(dbPath)
	}
	reorgEvents := []analytics.ReorgEvent{}
	if err == nil && detail != nil {
		reorgEvents = detail.ReorgEvents
		out["reorg_summary"] = detail.ReorgSummary
		out["analytics_enabled"] = detail.Exists
	} else {
		out["analytics_enabled"] = false
		if err != nil {
			out["analytics_error"] = err.Error()
		}
	}
	// Cap for UI table; full list remains on /api/analytics/summary.
	if len(reorgEvents) > 40 {
		reorgEvents = reorgEvents[len(reorgEvents)-40:]
	}
	out["recent_reorgs"] = reorgEvents
	out["hints"] = []string{
		"Mainnet and reboot testnet each keep their own chain folder and analytics DB.",
		"Enable Analytics sidecar at setup to persist reorg branch details across restarts.",
		"Tip-hash divergence needs connected peers that recently exchanged headers with this node.",
	}
	return out
}

func classifyPeerTip(localH int64, localHash string, peerH int64, peerHash string) (status, detail string) {
	if peerH < 0 {
		return "unknown", "Peer has not advertised a header tip yet."
	}
	if localH < 0 {
		return "unknown", "Local tip not ready."
	}
	if peerHash != "" && localHash != "" && peerHash == localHash {
		return "aligned", "Same tip hash as this node."
	}
	if peerHash != "" && localHash != "" && peerH == localH && peerHash != localHash {
		return "diverged", "Same height, different tip hash: competing fork tip or stale peer view."
	}
	if peerH > localH {
		if peerHash != "" && localHash != "" && peerHash != localHash {
			return "ahead", "Peer is ahead; tip hash differs: may be a heavier fork this node has not adopted yet."
		}
		return "ahead", "Peer header height is ahead of this node."
	}
	if peerH < localH {
		return "behind", "Peer is behind this node's tip."
	}
	if peerHash == "" {
		return "aligned", "Same height; peer tip hash not observed yet."
	}
	return "aligned", "Heights match."
}

func rpcResultMap(res map[string]interface{}) map[string]any {
	if res == nil {
		return nil
	}
	if err := res["error"]; err != nil {
		switch e := err.(type) {
		case string:
			if e != "" {
				return nil
			}
		case map[string]interface{}:
			return nil
		}
	}
	raw := res["result"]
	if m, ok := raw.(map[string]any); ok {
		return m
	}
	if m, ok := raw.(map[string]interface{}); ok {
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	return nil
}

func rpcResultArray(res map[string]interface{}) []any {
	if res == nil {
		return nil
	}
	if err := res["error"]; err != nil {
		if s, ok := err.(string); ok && s != "" {
			return nil
		}
		if _, ok := err.(map[string]interface{}); ok {
			return nil
		}
	}
	switch v := res["result"].(type) {
	case []any:
		return v
	default:
		// JSON decode may nest differently when result was re-marshaled.
		b, err := json.Marshal(res["result"])
		if err != nil {
			return nil
		}
		var arr []any
		if json.Unmarshal(b, &arr) == nil {
			return arr
		}
		return nil
	}
}

func asInt64Def(v any, def int64) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case json.Number:
		n, err := x.Int64()
		if err == nil {
			return n
		}
	case string:
		var n int64
		if _, err := fmt.Sscanf(x, "%d", &n); err == nil {
			return n
		}
	}
	return def
}
