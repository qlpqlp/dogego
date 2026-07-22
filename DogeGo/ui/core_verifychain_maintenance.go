// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
)

var maintenanceVerifyChainLevels = []int{2, 4}

func probeMaintenanceVerifyChain(out *CoreMaintenanceResult, ep CoreParityEndpoints, coreAvailable bool, invoke func(string, []json.RawMessage) map[string]interface{}, ibd bool) {
	for _, level := range maintenanceVerifyChainLevels {
		appendDogegoVerifyChain(out, invoke, level, ibd)
		if coreAvailable && !ibd {
			compareVerifyChainWithCore(out, ep, invoke, level)
		}
	}
}

func appendDogegoVerifyChain(out *CoreMaintenanceResult, invoke func(string, []json.RawMessage) map[string]interface{}, level int, ibd bool) {
	name := fmt.Sprintf("verifychain(%d,0)", level)
	verify, err := invokeDogeGoRPCAny(invoke, "verifychain", []any{level, 0})
	if err != nil {
		if level == 4 {
			out.Issues = append(out.Issues, "verifychain_failed")
		}
		out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: name, Status: "issue", Note: err.Error()})
		return
	}
	verifyOK := boolFromAny(verify)
	if !verifyOK {
		if ibd {
			out.Warnings = append(out.Warnings, fmt.Sprintf("verifychain_%d_not_true_during_ibd", level))
			out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: name, Status: "warning", DogeGo: verify})
		} else {
			if level == 4 {
				out.Issues = append(out.Issues, "verifychain_not_true")
			} else {
				out.Warnings = append(out.Warnings, fmt.Sprintf("verifychain_%d_not_true", level))
			}
			out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: name, Status: "issue", DogeGo: verify})
		}
		return
	}
	out.Checks = append(out.Checks, CoreMaintenanceCheck{Name: name, Status: "ok", DogeGo: verify})
}

func compareVerifyChainWithCore(out *CoreMaintenanceResult, ep CoreParityEndpoints, invoke func(string, []json.RawMessage) map[string]interface{}, level int) {
	name := fmt.Sprintf("verifychain(%d,0)", level)
	dgVerify, dgErr := invokeDogeGoRPCAny(invoke, "verifychain", []any{level, 0})
	coreVerify, coreErr := invokeExternalRPCAny(ep.Addr, ep.User, ep.Pass, "verifychain", []any{level, 0})
	if dgErr != nil || coreErr != nil {
		if coreErr != nil {
			out.Notes = append(out.Notes, fmt.Sprintf("core_verifychain_%d_compare_skipped", level))
		}
		return
	}
	dgOK := boolFromAny(dgVerify)
	coreOK := boolFromAny(coreVerify)
	note := ""
	st := "ok"
	switch {
	case dgOK && coreOK:
		out.Notes = append(out.Notes, fmt.Sprintf("verifychain_%d_core_ok", level))
		note = "both nodes report true"
	case !coreOK:
		st = "warning"
		out.Warnings = append(out.Warnings, fmt.Sprintf("core_verifychain_%d_not_true", level))
		note = "Core may still be syncing"
	case !dgOK && coreOK:
		st = "warning"
		out.Warnings = append(out.Warnings, fmt.Sprintf("verifychain_%d_dogego_false_core_true", level))
	default:
		st = "warning"
		out.Warnings = append(out.Warnings, fmt.Sprintf("verifychain_%d_mismatch", level))
	}
	out.Checks = append(out.Checks, CoreMaintenanceCheck{
		Name: name + ".core_compare", Status: st,
		DogeGo: dgOK, Core: coreOK, Note: note,
	})
}
