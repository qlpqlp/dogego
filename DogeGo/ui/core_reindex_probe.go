// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"dogego/config"
)

// CoreReindexCheck is one reindex workflow row (Milestone E).
type CoreReindexCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warning, issue, skipped
	Value  any    `json:"value,omitempty"`
	Note   string `json:"note,omitempty"`
}

// CoreReindexProbeResult is returned by GET /api/core-reindex-probe (check-only; no destructive RPC).
type CoreReindexProbeResult struct {
	OK             bool               `json:"ok"`
	Network        string             `json:"network"`
	Blocks         int64              `json:"blocks,omitempty"`
	IBD            bool               `json:"ibd"`
	CoreConfigured bool               `json:"core_configured,omitempty"`
	CoreAvailable  bool               `json:"core_available,omitempty"`
	CoreRPCAddr    string             `json:"core_rpc_addr,omitempty"`
	CheckedAt      string             `json:"checked_at"`
	Index          map[string]any     `json:"index,omitempty"`
	Checks         []CoreReindexCheck `json:"checks"`
	Issues         []string           `json:"issues,omitempty"`
	Warnings       []string           `json:"warnings,omitempty"`
	Notes          []string           `json:"notes,omitempty"`
	Hint           string             `json:"hint,omitempty"`
}

var coreReindexRequiredMethods = []string{
	"reindextx", "reindexblockfilters", "pruneblockchain", "getindexinfo", "verifychain",
}

// ProbeCoreReindex mirrors scripts/core_reindex_prune_workflow.ps1 (check-only).
func ProbeCoreReindex(network string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) CoreReindexProbeResult {
	ep := ResolveCoreParityEndpoints(network, conf)
	out := CoreReindexProbeResult{
		Network:        strings.TrimSpace(network),
		CoreRPCAddr:    ep.Addr,
		CoreConfigured: CoreCompareEnabled(network, conf),
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		Hint:           "Milestone E reindex workflow - check-only. Solo testnet: index lag during IBD/sync is OK. When Core is configured and caught up, compares getindexinfo tx/filter/coinstats vs Core. Mirrors scripts/core_reindex_prune_workflow.ps1 (no reindextx on mainnet).",
		Notes:          []string{"reindex_skipped_check_only"},
	}
	if invoke == nil {
		out.Issues = append(out.Issues, "dogego_rpc_not_ready")
		return out
	}

	rpcInfo, rpcErr := invokeDogeGoRPC(invoke, "getrpcinfo", nil)
	if rpcErr != nil {
		out.Issues = append(out.Issues, "getrpcinfo_failed")
		out.Checks = append(out.Checks, CoreReindexCheck{Name: "getrpcinfo", Status: "issue", Note: rpcErr.Error()})
	} else {
		methods, _ := rpcInfo["method"].(map[string]interface{})
		for _, m := range coreReindexRequiredMethods {
			st := "ok"
			if methods == nil || methods[m] == nil {
				out.Issues = append(out.Issues, "rpc_method_missing_"+m)
				st = "issue"
			}
			out.Checks = append(out.Checks, CoreReindexCheck{Name: "rpc." + m, Status: st})
		}
	}

	info, infoErr := invokeDogeGoRPC(invoke, "getblockchaininfo", nil)
	idx, idxErr := invokeDogeGoRPC(invoke, "getindexinfo", nil)
	if infoErr != nil || idxErr != nil {
		out.Issues = append(out.Issues, "rpc_chain_or_index_failed")
		if infoErr != nil {
			out.Checks = append(out.Checks, CoreReindexCheck{Name: "getblockchaininfo", Status: "issue", Note: infoErr.Error()})
		}
		if idxErr != nil {
			out.Checks = append(out.Checks, CoreReindexCheck{Name: "getindexinfo", Status: "issue", Note: idxErr.Error()})
		}
		out.OK = len(out.Issues) == 0
		return out
	}
	if blk, ok := intFromAny(info["blocks"]); ok {
		out.Blocks = blk
	}
	if ibd, ok := info["initialblockdownload"].(bool); ok {
		out.IBD = ibd
	}
	out.Index = idx
	out.Checks = append(out.Checks, CoreReindexCheck{Name: "getindexinfo", Status: "ok", Value: idx})

	if txIdx, ok := idx["txindex"].(map[string]interface{}); ok {
		if synced, _ := txIdx["synced"].(bool); !synced {
			if reindexSyncing(info, out.IBD) {
				out.Notes = append(out.Notes, "txindex_catching_up_during_sync")
			} else {
				out.Warnings = append(out.Warnings, "txindex_not_synced")
				out.Checks = append(out.Checks, CoreReindexCheck{Name: "txindex.synced", Status: "warning", Value: false})
			}
		}
	}
	if basic := indexSection(idx, "basic block filter", "basic"); basic != nil {
		if synced, _ := basic["synced"].(bool); !synced {
			if reindexSyncing(info, out.IBD) {
				out.Notes = append(out.Notes, "block_filter_index_catching_up_during_sync")
			} else {
				out.Warnings = append(out.Warnings, "block_filter_index_not_synced")
				out.Checks = append(out.Checks, CoreReindexCheck{Name: "basic.synced", Status: "warning", Value: false})
			}
		}
	}

	if out.CoreConfigured {
		out.CoreAvailable = probeCoreReachable(ep)
	}
	if out.CoreAvailable && !out.IBD && !reindexSyncing(info, out.IBD) {
		coreIdx, coreIdxErr := invokeExternalRPC(ep.Addr, ep.User, ep.Pass, "getindexinfo", nil)
		if coreIdxErr != nil {
			out.Notes = append(out.Notes, "core_getindexinfo_compare_skipped")
		} else {
			applyIndexInfoCoreCompareToReindex(&out, idx, coreIdx)
		}
	}

	out.OK = len(out.Issues) == 0
	return out
}

func reindexSyncing(info map[string]any, ibd bool) bool {
	if ibd {
		return true
	}
	if info == nil {
		return false
	}
	blocks, bOK := intFromAny(info["blocks"])
	headers, hOK := intFromAny(info["headers"])
	return bOK && hOK && blocks < headers
}

func connectLagMax() int64 {
	if v := strings.TrimSpace(os.Getenv("DOGEGO_CONNECT_LAG_MAX")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			return n
		}
	}
	return 2048
}

func connectLagFromInfo(info map[string]any) (int64, bool) {
	if info == nil {
		return 0, false
	}
	if lag, ok := intFromAny(info["dogego_connect_lag"]); ok {
		return lag, true
	}
	if lag, ok := intFromAny(info["dogego_stored_bodies_ahead_connect"]); ok {
		return lag, true
	}
	blocks, bOK := intFromAny(info["blocks"])
	cont, cOK := intFromAny(info["dogego_contiguous_raw_height"])
	if bOK && cOK && cont >= blocks {
		return cont - blocks, true
	}
	return 0, false
}

func connectCatchUpBoostNote(info map[string]any) string {
	if info == nil {
		return ""
	}
	var parts []string
	if passes, ok := intFromAny(info["dogego_connect_catch_up_passes"]); ok && passes > 0 {
		parts = append(parts, fmt.Sprintf("passes=%d", passes))
	}
	if batch, ok := intFromAny(info["dogego_connect_catch_up_batch"]); ok && batch > 0 {
		parts = append(parts, fmt.Sprintf("batch=%d", batch))
	}
	if ms, ok := intFromAny(info["dogego_connect_catch_up_interval_ms"]); ok && ms > 0 {
		parts = append(parts, fmt.Sprintf("interval=%dms", ms))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func populateConnectCatchUpFields(out *CoreRestartResumeResult, info map[string]any) {
	if out == nil || info == nil {
		return
	}
	if passes, ok := intFromAny(info["dogego_connect_catch_up_passes"]); ok && passes > 0 {
		out.ConnectCatchUpPasses = int(passes)
	}
	if batch, ok := intFromAny(info["dogego_connect_catch_up_batch"]); ok && batch > 0 {
		out.ConnectCatchUpBatch = int(batch)
	}
	if ms, ok := intFromAny(info["dogego_connect_catch_up_interval_ms"]); ok && ms > 0 {
		out.ConnectCatchUpIntervalMs = ms
	}
}

func appendConnectLagCheck(out *CoreRestartResumeResult, info map[string]any) {
	if out == nil {
		return
	}
	lag, ok := connectLagFromInfo(info)
	if !ok {
		return
	}
	out.ConnectLag = lag
	populateConnectCatchUpFields(out, info)
	maxLag := connectLagMax()
	if lag <= maxLag {
		note := fmt.Sprintf("max %d", maxLag)
		if boost := connectCatchUpBoostNote(info); boost != "" {
			note += " · boost " + boost
		}
		out.Checks = append(out.Checks, CoreRestartResumeCheck{
			Name: "connect_lag", Status: "ok", Value: lag,
			Note: note,
		})
		return
	}
	if out.IBD {
		out.Warnings = append(out.Warnings, "connect_lag_above_threshold")
		out.Checks = append(out.Checks, CoreRestartResumeCheck{
			Name: "connect_lag", Status: "warning", Value: lag,
			Note: connectLagCheckNote(info, maxLag, true),
		})
	} else {
		out.Issues = append(out.Issues, "connect_lag_above_threshold")
		out.Checks = append(out.Checks, CoreRestartResumeCheck{
			Name: "connect_lag", Status: "issue", Value: lag,
			Note: connectLagCheckNote(info, maxLag, false),
		})
	}
}

func connectLagCheckNote(info map[string]any, maxLag int64, ibd bool) string {
	note := fmt.Sprintf("above %d", maxLag)
	if ibd {
		note += " during IBD"
	}
	if boost := connectCatchUpBoostNote(info); boost != "" {
		note += " (" + boost + ")"
	}
	return note
}
