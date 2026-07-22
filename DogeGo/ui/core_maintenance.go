// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dogego/config"
)

// CoreMaintenanceCheck is one maintenance workflow row (Milestone E).
type CoreMaintenanceCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warning, issue, skipped
	DogeGo any    `json:"dogego,omitempty"`
	Core   any    `json:"core,omitempty"`
	Note   string `json:"note,omitempty"`
}

// CoreMaintenanceResult is returned by GET /api/core-maintenance.
type CoreMaintenanceResult struct {
	OK            bool                   `json:"ok"`
	Network       string                 `json:"network"`
	Blocks        int64                  `json:"blocks,omitempty"`
	Headers       int64                  `json:"headers,omitempty"`
	IBD           bool                   `json:"ibd"`
	CoreAvailable bool                   `json:"core_available"`
	CoreConfigured bool                  `json:"core_configured,omitempty"`
	CoreRPCAddr   string                 `json:"core_rpc_addr,omitempty"`
	CheckedAt     string                 `json:"checked_at"`
	Checks        []CoreMaintenanceCheck `json:"checks"`
	Issues        []string               `json:"issues,omitempty"`
	Warnings      []string               `json:"warnings,omitempty"`
	Notes         []string               `json:"notes,omitempty"`
	Hint          string                 `json:"hint,omitempty"`
}

// ProbeCoreMaintenance runs Core-style maintenance RPC checks on DogeGo (+ optional Core compare).
func ProbeCoreMaintenance(network string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) CoreMaintenanceResult {
	ep := ResolveCoreParityEndpoints(network, conf)
	out := CoreMaintenanceResult{
		Network:        strings.TrimSpace(network),
		CoreRPCAddr:    ep.Addr,
		CoreConfigured: CoreCompareEnabled(network, conf),
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		Hint:           "Milestone E maintenance probe - mirrors scripts/core_maintenance_workflow.ps1. Solo testnet: OK while syncing; Core side-by-side optional (Settings → Advanced). When Core is configured and caught up, compares verifychain levels 2+4, getindexinfo, getchaintxstats window, and getblockfilter at shared tip.",
	}
	if invoke == nil {
		out.Issues = append(out.Issues, "dogego_rpc_not_ready")
		return out
	}
	info, err := invokeDogeGoRPC(invoke, "getblockchaininfo", nil)
	if err != nil {
		out.Issues = append(out.Issues, "rpc_unreachable")
		out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: "getblockchaininfo", Status: "issue", Note: err.Error()})
		return out
	}
	out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: "getblockchaininfo", Status: "ok", DogeGo: info})
	if blk, ok := intFromAny(info["blocks"]); ok {
		out.Blocks = blk
	}
	if hdr, ok := intFromAny(info["headers"]); ok {
		out.Headers = hdr
	}
	if ibd, ok := info["initialblockdownload"].(bool); ok {
		out.IBD = ibd
	}
	if out.Blocks > out.Headers && out.Headers >= 0 {
		if out.IBD {
			out.Warnings = append(out.Warnings, "blocks_exceed_headers_during_ibd")
			out.Checks = append(out.Checks, CoreMaintenanceCheck{
				Name: "blocks_vs_headers", Status: "warning",
				DogeGo: fmt.Sprintf("blocks=%d headers=%d", out.Blocks, out.Headers),
			})
		} else {
			out.Issues = append(out.Issues, "blocks_exceed_headers")
			out.Checks = append(out.Checks, CoreMaintenanceCheck{
				Name: "blocks_vs_headers", Status: "issue",
				DogeGo: fmt.Sprintf("blocks=%d headers=%d", out.Blocks, out.Headers),
			})
		}
	}

	if out.CoreConfigured {
		out.CoreAvailable = probeCoreReachable(ep)
	}

	probeMaintenanceVerifyChain(&out, ep, out.CoreAvailable, invoke, out.IBD)

	idx, idxErr := invokeDogeGoRPC(invoke, "getindexinfo", nil)
	if idxErr != nil {
		out.Issues = append(out.Issues, "getindexinfo_failed")
		out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: "getindexinfo", Status: "issue", Note: idxErr.Error()})
	} else {
		st := "ok"
		if _, hasTx := idx["txindex"]; !hasTx {
			out.Warnings = append(out.Warnings, "getindexinfo_missing_txindex")
			st = "warning"
		}
		if basic := indexSection(idx, "basic block filter", "basic"); basic != nil {
			if synced, _ := basic["synced"].(bool); !synced {
				if out.IBD {
					out.Notes = append(out.Notes, "block_filter_index_catching_up")
				} else {
					out.Warnings = append(out.Warnings, "block_filter_index_not_synced")
					if st == "ok" {
						st = "warning"
					}
				}
			}
		}
		out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: "getindexinfo", Status: st, DogeGo: idx})
	}

	stats, statsErr := invokeDogeGoRPC(invoke, "getchaintxstats", []any{24})
	if statsErr != nil {
		out.Warnings = append(out.Warnings, "getchaintxstats_failed")
		out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: "getchaintxstats(24)", Status: "warning", Note: statsErr.Error()})
	} else {
		st := "ok"
		if _, has := stats["window_tx_count"]; !has {
			out.Warnings = append(out.Warnings, "getchaintxstats_missing_window_tx_count")
			st = "warning"
		}
		if t, ok := intFromAny(stats["time"]); ok && t <= 0 {
			out.Warnings = append(out.Warnings, "getchaintxstats_time_zero")
			st = "warning"
		}
		out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: "getchaintxstats(24)", Status: st, DogeGo: stats})
	}

	if out.Blocks > 0 {
		best, bestErr := invokeDogeGoRPCAny(invoke, "getbestblockhash", nil)
		if bestErr != nil {
			if out.IBD {
				out.Notes = append(out.Notes, "getblockfilter_skipped_during_ibd")
			} else {
				out.Warnings = append(out.Warnings, "getblockfilter_failed")
			}
		} else {
			bestHash := strFromAny(best)
			filter, fErr := invokeDogeGoRPC(invoke, "getblockfilter", []any{bestHash})
			if fErr != nil {
				if out.IBD {
					out.Notes = append(out.Notes, "getblockfilter_skipped_during_ibd")
				} else {
					out.Warnings = append(out.Warnings, "getblockfilter_failed")
				}
			} else if filter == nil || strFromAny(filter["filter"]) == "" {
				if out.IBD || maintenanceSyncing(out) {
					out.Notes = append(out.Notes, "getblockfilter_empty_during_sync")
				} else {
					out.Warnings = append(out.Warnings, "getblockfilter_empty_at_tip")
					out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: "getblockfilter(tip)", Status: "warning", DogeGo: filter})
				}
			} else {
				out.Notes = append(out.Notes, "getblockfilter_ok_at_tip")
				out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: "getblockfilter(tip)", Status: "ok", DogeGo: map[string]any{"filter_len": len(strFromAny(filter["filter"]))}})
				if out.CoreAvailable && !out.IBD {
					compareBlockFilterWithCore(&out, ep, bestHash, filter)
				}
			}
		}
	}

	if out.CoreAvailable && statsErr == nil && !out.IBD {
		coreStats, coreErr := invokeExternalRPC(ep.Addr, ep.User, ep.Pass, "getchaintxstats", []any{24})
		if coreErr != nil {
			out.Notes = append(out.Notes, "core_chaintxstats_compare_skipped")
		} else {
			dgWin, dgOK := intFromAny(stats["window_tx_count"])
			coreWin, coreOK := intFromAny(coreStats["window_tx_count"])
			if !coreOK {
				coreWin, coreOK = intFromAny(coreStats["txcount"])
			}
			if dgOK && coreOK {
				delta := abs64(dgWin - coreWin)
				note := fmt.Sprintf("window_tx_count delta=%d", delta)
				if delta > 500 {
					out.Warnings = append(out.Warnings, fmt.Sprintf("chaintxstats_window_delta_%d", delta))
					out.Checks = append(out.Checks, CoreMaintenanceCheck{
						Name: "chaintxstats_window", Status: "warning",
						DogeGo: dgWin, Core: coreWin, Note: note,
					})
				} else {
					out.Notes = append(out.Notes, "chaintxstats_window_aligned")
					out.Checks = append(out.Checks, CoreMaintenanceCheck{
						Name: "chaintxstats_window", Status: "ok",
						DogeGo: dgWin, Core: coreWin, Note: note,
					})
				}
			}
		}
	}

	if out.CoreAvailable && idxErr == nil && !out.IBD {
		coreIdx, coreIdxErr := invokeExternalRPC(ep.Addr, ep.User, ep.Pass, "getindexinfo", nil)
		if coreIdxErr != nil {
			out.Notes = append(out.Notes, "core_getindexinfo_compare_skipped")
		} else {
			compareIndexInfoWithCore(&out, idx, coreIdx)
		}
	}

	out.OK = maintenanceOperationalOK(out)
	return out
}
