// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// chainTxData mirrors Core ChainTxData for GuessVerificationProgress (mainnet checkpoint).
type chainTxData struct {
	checkpointTime int64
	checkpointTx   int64
	txRate         float64
}

func chainTxDataForNetwork(network string) (chainTxData, bool) {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "main", "mainnet":
		return chainTxData{
			checkpointTime: 1705383360,
			checkpointTx:   226128837,
			txRate:         4.23,
		}, true
	default:
		return chainTxData{}, false
	}
}

// guessVerificationProgress estimates chain verification like Core (tx count vs projected total).
// chainTx is cumulative tx count at tip; tipTime is header tip unix time.
func guessVerificationProgress(data chainTxData, chainTx int64, tipTime int64) float64 {
	if chainTx <= 0 || tipTime <= 0 {
		return 0
	}
	now := time.Now().Unix()
	var fTxTotal float64
	if chainTx <= data.checkpointTx {
		fTxTotal = float64(data.checkpointTx) + float64(now-data.checkpointTime)*data.txRate
	} else {
		fTxTotal = float64(chainTx) + float64(now-tipTime)*data.txRate
	}
	if fTxTotal <= 0 {
		return 0
	}
	p := float64(chainTx) / fTxTotal
	if p > 1 {
		p = 1
	}
	if p < 0 {
		p = 0
	}
	return p
}

// FormatSyncETA returns a Core-style remaining-time string from blocks behind and download rate.
func FormatSyncETA(blocksBehind int64, blocksPerMin float64) string {
	if blocksBehind <= 0 {
		return ""
	}
	if blocksPerMin <= 0 {
		if blocksBehind > 500_000 {
			return "more than a week (rate unknown yet)"
		}
		return "estimating download rate…"
	}
	minutes := float64(blocksBehind) / blocksPerMin
	secs := int64(minutes * 60)
	return formatDurationHuman(secs)
}

func formatDurationHuman(secs int64) string {
	if secs < 60 {
		return "less than a minute"
	}
	if secs < 3600 {
		m := (secs + 29) / 60
		if m == 1 {
			return "about 1 minute"
		}
		return fmt.Sprintf("about %d minutes", m)
	}
	if secs < 86400 {
		h := (secs + 1800) / 3600
		if h == 1 {
			return "about 1 hour"
		}
		return fmt.Sprintf("about %d hours", h)
	}
	d := (secs + 43200) / 86400
	if d == 1 {
		return "about 1 day"
	}
	if d < 14 {
		return fmt.Sprintf("about %d days", d)
	}
	w := (d + 3) / 7
	if w < 8 {
		return fmt.Sprintf("about %d weeks", w)
	}
	mo := (d + 15) / 30
	if mo < 24 {
		return fmt.Sprintf("about %d months", mo)
	}
	y := (d + 183) / 365
	if y == 1 {
		return "about 1 year"
	}
	return fmt.Sprintf("about %d years", y)
}

// SyncStatusLine is one sentence for UI splash (Core-style status text).
func SyncStatusLine(nodeMode, syncPhase string, tip, contiguous int64, bodyPct float64, behind int64, eta string, mempool int) string {
	if strings.ToLower(nodeMode) == "spv" {
		if tip >= 0 {
			return fmt.Sprintf("Synchronizing headers (height %s)", formatInt(tip))
		}
		return "Synchronizing headers…"
	}
	switch syncPhase {
	case "awaiting_genesis_block":
		return "Downloading block bodies - waiting for genesis block…"
	case "forward_block_ibd":
		pct := int(math.Round(bodyPct * 100))
		if pct > 100 {
			pct = 100
		}
		if behind > 0 && eta != "" {
			return fmt.Sprintf("Synchronizing blocks… %d%% (%s behind, ~%s left)", pct, formatInt(behind), eta)
		}
		if behind > 0 {
			return fmt.Sprintf("Synchronizing blocks… %d%% (%s blocks behind header tip)", pct, formatInt(behind))
		}
		return fmt.Sprintf("Synchronizing blocks… %d%%", pct)
	case "block_chain_connected":
		if bodyPct < 0.999 || behind > 0 {
			pct := int(math.Round(bodyPct * 100))
			if pct > 100 {
				pct = 100
			}
			if behind > 0 && eta != "" {
				return fmt.Sprintf("Synchronizing blocks… %d%% (%s behind, ~%s left)", pct, formatInt(behind), eta)
			}
			if behind > 0 {
				return fmt.Sprintf("Synchronizing blocks… %d%% (%s blocks behind header tip)", pct, formatInt(behind))
			}
			return fmt.Sprintf("Synchronizing blocks… %d%%", pct)
		}
		if mempool > 0 {
			return fmt.Sprintf("Up to date · %s mempool transactions", formatInt(int64(mempool)))
		}
		return "Up to date"
	default:
		if tip < 0 {
			return "Connecting to network…"
		}
		if contiguous < tip {
			return fmt.Sprintf("Header tip %s · downloading block bodies", formatInt(tip))
		}
		return fmt.Sprintf("Header tip %s", formatInt(tip))
	}
}

func formatInt(n int64) string {
	if n < 0 {
		return "-"
	}
	return fmt.Sprintf("%d", n)
}

// SyncHealthAssessment returns an operator health label and whether sync looks on track.
func SyncHealthAssessment(syncPhase string, headers, chainActive, behind int64, blocksPerMin float64, lastBlockStoredUnix int64, headerRecovery string, headerCatchUpPending bool) (health string, ok bool) {
	if syncPhase == "forward_block_ibd" && behind > 50_000 && chainActive >= 0 {
		if blocksPerMin > 0.02 || (lastBlockStoredUnix > 0 && time.Now().Unix()-lastBlockStoredUnix < 900) {
			return "forward_ibd_active", true
		}
	}
	if strings.TrimSpace(headerRecovery) != "" || headerCatchUpPending {
		recentBody := blocksPerMin > 0.05
		if !recentBody && lastBlockStoredUnix > 0 && time.Now().Unix()-lastBlockStoredUnix < 900 {
			recentBody = true
		}
		if recentBody {
			return "headers_catching_up", true
		}
		if strings.TrimSpace(headerRecovery) != "" || headerCatchUpPending {
			return "header_attention", false
		}
	}
	switch syncPhase {
	case "block_chain_connected", "caught_up":
		if behind > 32 {
			if blocksPerMin > 0.1 {
				return "forward_ibd_active", true
			}
			if lastBlockStoredUnix > 0 && time.Now().Unix()-lastBlockStoredUnix < 900 {
				return "forward_ibd_active", true
			}
			return "syncing", true
		}
		return "healthy", true
	case "awaiting_genesis_block":
		if lastBlockStoredUnix > 0 || blocksPerMin > 0 {
			return "forward_ibd_active", true
		}
		return "forward_ibd_starting", true
	case "forward_block_ibd":
		if chainActive < 0 {
			if blocksPerMin > 0.1 || (lastBlockStoredUnix > 0 && time.Now().Unix()-lastBlockStoredUnix < 900) {
				return "forward_ibd_active", true
			}
			return "forward_ibd_starting", true
		}
		if behind > 512 {
			if blocksPerMin > 0.1 {
				return "forward_ibd_active", true
			}
			if lastBlockStoredUnix > 0 && time.Now().Unix()-lastBlockStoredUnix < 900 {
				return "forward_ibd_active", true
			}
			if chainActive >= 0 && chainActive < 128 {
				return "forward_ibd_starting", true
			}
			return "forward_ibd_stalled", false
		}
		return "syncing", true
	default:
		if behind > 0 {
			return "syncing", true
		}
		return "starting", true
	}
}

// ConnectedVerificationProgress is chainActive (UTXO/connect) coverage vs header tip (0..1).
func ConnectedVerificationProgress(tip, chainActive int64) float64 {
	return BodyVerificationProgress(tip, chainActive)
}

// BodyVerificationProgress is contiguous stored block-body coverage vs header tip (0..1).
func BodyVerificationProgress(tip, contiguous int64) float64 {
	if tip < 0 {
		return 0
	}
	want := int(tip) + 1
	have := int(contiguous) + 1
	if contiguous < 0 {
		have = 0
	}
	if want <= 0 {
		return 1
	}
	if have >= want {
		return 1
	}
	return float64(have) / float64(want)
}

// mergeDogegoRawSyncDiagnostics copies IBD stall/timeout fields from dogego_raw_sync to top-level RPC keys.
func mergeDogegoRawSyncDiagnostics(res map[string]interface{}, prog map[string]interface{}) {
	if res == nil || prog == nil {
		return
	}
	if v, ok := prog["block_stalling_timeout_sec"].(int64); ok {
		res["dogego_block_stalling_timeout_sec"] = v
	}
	if v, ok := prog["block_download_timeout_sec"].(int64); ok {
		res["dogego_block_download_timeout_sec"] = v
	}
	if v, ok := prog["last_block_stall_peer"].(string); ok && v != "" {
		res["dogego_last_block_stall_peer"] = v
		if t, ok := prog["last_block_stall_at"].(int64); ok && t > 0 {
			res["dogego_last_block_stall_at"] = t
		}
	}
	if v, ok := prog["last_block_download_timeout_peer"].(string); ok && v != "" {
		res["dogego_last_block_download_timeout_peer"] = v
		if t, ok := prog["last_block_download_timeout_at"].(int64); ok && t > 0 {
			res["dogego_last_block_download_timeout_at"] = t
		}
	}
	if v, ok := prog["frontier_stalling_since"].(int64); ok && v > 0 {
		res["dogego_frontier_stalling_since"] = v
	}
	if v, ok := prog["max_blocks_in_transit_per_peer"].(int); ok && v > 0 {
		res["dogego_max_blocks_in_transit_per_peer"] = v
	}
	if v, ok := prog["lane_in_flight"].(map[string]int); ok && len(v) > 0 {
		res["dogego_lane_in_flight"] = v
	}
	if v, ok := prog["raw_blocks_ahead_of_contiguous"].(int64); ok && v > 0 {
		res["dogego_raw_blocks_ahead_of_contiguous"] = v
	}
	if v, ok := prog["connect_lag"].(int64); ok && v > 0 {
		res["dogego_connect_lag"] = v
	}
	if v, ok := prog["connect_catch_up_min_lag"].(int64); ok && v > 0 {
		res["dogego_connect_catch_up_min_lag"] = v
	} else if v, ok := prog["connect_catch_up_min_lag"].(float64); ok && v > 0 {
		res["dogego_connect_catch_up_min_lag"] = int64(v)
	}
	if v, ok := prog["connect_catch_up_passes"].(int); ok && v > 0 {
		res["dogego_connect_catch_up_passes"] = v
	} else if v, ok := prog["connect_catch_up_passes"].(int64); ok && v > 0 {
		res["dogego_connect_catch_up_passes"] = int(v)
	}
	if v, ok := prog["connect_catch_up_batch"].(int); ok && v > 0 {
		res["dogego_connect_catch_up_batch"] = v
	} else if v, ok := prog["connect_catch_up_batch"].(int64); ok && v > 0 {
		res["dogego_connect_catch_up_batch"] = int(v)
	}
	if v, ok := prog["connect_catch_up_interval_ms"].(int64); ok && v > 0 {
		res["dogego_connect_catch_up_interval_ms"] = v
	}
	if v, ok := prog["body_ibd_header_paused"].(bool); ok && v {
		res["dogego_body_ibd_header_paused"] = true
	}
	if v, ok := prog["body_ibd_eta_minutes"].(int64); ok && v > 0 {
		res["dogego_body_ibd_eta_minutes"] = v
	}
}
