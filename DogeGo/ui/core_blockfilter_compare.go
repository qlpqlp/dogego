// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// compareBlockFilterWithCore compares BIP158 basic filter at blockHash when Core shares the same tip.
func compareBlockFilterWithCore(out *CoreMaintenanceResult, ep CoreParityEndpoints, blockHash string, dgFilter map[string]any) {
	if out == nil || blockHash == "" || dgFilter == nil {
		return
	}
	dgHex := strFromAny(dgFilter["filter"])
	if dgHex == "" {
		return
	}
	coreInfo, err := invokeExternalRPC(ep.Addr, ep.User, ep.Pass, "getblockchaininfo", nil)
	if err != nil {
		out.Notes = append(out.Notes, "core_getblockfilter_compare_skipped")
		return
	}
	coreBest := strFromAny(coreInfo["bestblockhash"])
	if coreBest == "" || coreBest != blockHash {
		out.Notes = append(out.Notes, "getblockfilter_compare_skipped_tip_mismatch")
		return
	}
	coreFilter, err := invokeExternalRPC(ep.Addr, ep.User, ep.Pass, "getblockfilter", []any{blockHash})
	if err != nil {
		out.Notes = append(out.Notes, "core_getblockfilter_compare_skipped")
		return
	}
	coreHex := strFromAny(coreFilter["filter"])
	st := "ok"
	note := "BIP158 basic filter at shared tip"
	if coreHex == "" {
		st = "warning"
		out.Warnings = append(out.Warnings, "core_getblockfilter_empty_at_tip")
		note = "Core returned empty filter at tip"
	} else if dgHex != coreHex {
		st = "warning"
		out.Warnings = append(out.Warnings, "getblockfilter_tip_mismatch")
		note = "filter hex mismatch at shared tip"
	} else {
		out.Notes = append(out.Notes, "getblockfilter_tip_aligned")
	}
	out.Checks = append(out.Checks, CoreMaintenanceCheck{
		Name: "getblockfilter(tip).core_compare", Status: st,
		DogeGo: map[string]any{"filter_len": len(dgHex), "blockhash": blockHash},
		Core:   map[string]any{"filter_len": len(coreHex), "blockhash": coreBest},
		Note:   note,
	})
}
