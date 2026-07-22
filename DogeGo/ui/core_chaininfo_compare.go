// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "fmt"

// compareChainInfoConsensusFields adds chainwork and mediantime rows from getblockchaininfo.
// When bestblockhash matches, mismatches indicate consensus divergence at the shared tip.
func compareChainInfoConsensusFields(out *CoreCompareResult, dg, core map[string]any) bool {
	if out == nil || dg == nil || core == nil {
		return true
	}
	sameTip := sharedChainTip(dg, core)
	allMatch := true

	dgCW := strFromAny(dg["chainwork"])
	coreCW := strFromAny(core["chainwork"])
	if dgCW != "" && coreCW != "" {
		match := dgCW == coreCW
		note := ""
		if !match {
			if sameTip {
				note = "chainwork mismatch at shared tip (consensus divergence)"
				allMatch = false
			} else {
				note = "tips may differ during catch-up"
				match = true
			}
		}
		out.Fields = append(out.Fields, CoreCompareField{
			Name: "chainwork", DogeGo: dgCW, Core: coreCW, Match: match, Note: note,
		})
	}

	dgMT, dgMOK := intFromAny(dg["mediantime"])
	coreMT, coreMOK := intFromAny(core["mediantime"])
	if dgMOK && coreMOK {
		match := dgMT == coreMT
		note := ""
		if !match {
			if sameTip {
				note = "mediantime mismatch at shared tip"
				allMatch = false
			} else {
				note = "tips may differ during catch-up"
				match = true
			}
		}
		out.Fields = append(out.Fields, CoreCompareField{
			Name: "mediantime", DogeGo: dgMT, Core: coreMT, Match: match, Note: note,
		})
	}

	appendVerificationProgressCompare(out, dg, core)

	return allMatch
}

func appendVerificationProgressCompare(out *CoreCompareResult, dg, core map[string]any) {
	if out == nil || dg == nil || core == nil {
		return
	}
	if boolFromAny(dg["initialblockdownload"]) || boolFromAny(core["initialblockdownload"]) {
		return
	}
	dgVP, dgOK := floatFromAny(dg["verificationprogress"])
	coreVP, coreOK := floatFromAny(core["verificationprogress"])
	if !dgOK || !coreOK {
		return
	}
	delta := dgVP - coreVP
	if delta < 0 {
		delta = -delta
	}
	match := delta <= 0.05 || (dgVP >= 0.999 && coreVP >= 0.999)
	note := fmt.Sprintf("delta=%.4f", delta)
	if !match {
		note = "verification progress diverged while caught up"
	}
	out.Fields = append(out.Fields, CoreCompareField{
		Name: "verificationprogress", DogeGo: dgVP, Core: coreVP, Match: match, Note: note,
	})
}

func sharedChainTip(dg, core map[string]any) bool {
	dgBest := strFromAny(dg["bestblockhash"])
	coreBest := strFromAny(core["bestblockhash"])
	return dgBest != "" && dgBest == coreBest
}
