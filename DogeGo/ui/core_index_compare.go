// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "fmt"

// indexSection returns the first present getindexinfo subsection (e.g. txindex, basic block filter).
func indexSection(idx map[string]any, keys ...string) map[string]any {
	if idx == nil {
		return nil
	}
	for _, key := range keys {
		if sub := asStringMap(idx[key]); sub != nil {
			return sub
		}
	}
	return nil
}

type indexInfoCoreRow struct {
	Name   string
	Status string
	DogeGo any
	Core   any
	Note   string
}

type indexInfoCoreCompare struct {
	Checks   []indexInfoCoreRow
	Warnings []string
	Notes    []string
}

func buildIndexInfoCoreCompare(dgIdx, coreIdx map[string]any) indexInfoCoreCompare {
	out := indexInfoCoreCompare{}
	if dgIdx == nil || coreIdx == nil {
		return out
	}
	maxBlockDelta := parityMaxDelta("DOGEGO_PARITY_MAX_BLOCK_DELTA", 500)

	compareIndexSynced := func(name string, dgSub, coreSub map[string]any) {
		dgHas := dgSub != nil
		coreHas := coreSub != nil
		if dgHas != coreHas {
			out.Warnings = append(out.Warnings, "getindexinfo_"+name+"_presence_mismatch")
			out.Checks = append(out.Checks, indexInfoCoreRow{
				Name: "getindexinfo." + name + ".presence", Status: "warning",
				DogeGo: dgHas, Core: coreHas,
			})
			return
		}
		if !dgHas {
			return
		}
		dgSynced := boolFromAny(dgSub["synced"])
		coreSynced := boolFromAny(coreSub["synced"])
		if dgSynced != coreSynced {
			out.Warnings = append(out.Warnings, "getindexinfo_"+name+"_synced_mismatch")
			out.Checks = append(out.Checks, indexInfoCoreRow{
				Name: "getindexinfo." + name + ".synced", Status: "warning",
				DogeGo: dgSynced, Core: coreSynced,
			})
		} else {
			out.Checks = append(out.Checks, indexInfoCoreRow{
				Name: "getindexinfo." + name + ".synced", Status: "ok",
				DogeGo: dgSynced, Core: coreSynced,
			})
		}
		dgH, dgHOK := intFromAny(dgSub["best_block_height"])
		coreH, coreHOK := intFromAny(coreSub["best_block_height"])
		if dgHOK && coreHOK {
			delta := abs64(dgH - coreH)
			note := fmt.Sprintf("best_block_height delta=%d", delta)
			st := "ok"
			if delta > maxBlockDelta {
				st = "warning"
				out.Warnings = append(out.Warnings, fmt.Sprintf("getindexinfo_%s_height_delta_%d", name, delta))
			} else {
				out.Notes = append(out.Notes, "getindexinfo_"+name+"_height_aligned")
			}
			out.Checks = append(out.Checks, indexInfoCoreRow{
				Name: "getindexinfo." + name + ".best_block_height", Status: st,
				DogeGo: dgH, Core: coreH, Note: note,
			})
		}
	}

	compareIndexSynced("txindex", indexSection(dgIdx, "txindex"), indexSection(coreIdx, "txindex"))
	compareIndexSynced("basic", indexSection(dgIdx, "basic block filter", "basic"), indexSection(coreIdx, "basic block filter", "basic"))

	dgCoin := indexSection(dgIdx, "coinstatsindex")
	coreCoin := indexSection(coreIdx, "coinstatsindex")
	if dgCoin != nil && coreCoin != nil {
		dgSynced := boolFromAny(dgCoin["synced"])
		coreSynced := boolFromAny(coreCoin["synced"])
		if dgSynced != coreSynced {
			out.Warnings = append(out.Warnings, "getindexinfo_coinstatsindex_synced_mismatch")
			out.Checks = append(out.Checks, indexInfoCoreRow{
				Name: "getindexinfo.coinstatsindex.synced", Status: "warning",
				DogeGo: dgSynced, Core: coreSynced,
			})
		}
		if dgSynced && coreSynced {
			dgHash := strFromAny(dgCoin["hash_serialized"])
			coreHash := strFromAny(coreCoin["hash_serialized"])
			if dgHash != "" && coreHash != "" {
				st := "ok"
				if dgHash != coreHash {
					st = "warning"
					out.Warnings = append(out.Warnings, "getindexinfo_coinstats_hash_mismatch")
				} else {
					out.Notes = append(out.Notes, "coinstatsindex_hash_aligned")
				}
				out.Checks = append(out.Checks, indexInfoCoreRow{
					Name: "getindexinfo.coinstatsindex.hash_serialized", Status: st,
					DogeGo: dgHash, Core: coreHash,
					Note: "when both coinstatsindex synced",
				})
			}
		}
	}
	return out
}

// compareIndexInfoWithCore adds maintenance checks for getindexinfo fields vs Dogecoin Core.
func compareIndexInfoWithCore(out *CoreMaintenanceResult, dgIdx, coreIdx map[string]any) {
	if out == nil {
		return
	}
	cmp := buildIndexInfoCoreCompare(dgIdx, coreIdx)
	out.Warnings = append(out.Warnings, cmp.Warnings...)
	out.Notes = append(out.Notes, cmp.Notes...)
	for _, row := range cmp.Checks {
		out.Checks = append(out.Checks, CoreMaintenanceCheck{
			Name: row.Name, Status: row.Status, DogeGo: row.DogeGo, Core: row.Core, Note: row.Note,
		})
	}
}

func applyIndexInfoCoreCompareToReindex(out *CoreReindexProbeResult, dgIdx, coreIdx map[string]any) {
	if out == nil {
		return
	}
	cmp := buildIndexInfoCoreCompare(dgIdx, coreIdx)
	out.Warnings = append(out.Warnings, cmp.Warnings...)
	out.Notes = append(out.Notes, cmp.Notes...)
	for _, row := range cmp.Checks {
		val := map[string]any{}
		if row.DogeGo != nil {
			val["dogego"] = row.DogeGo
		}
		if row.Core != nil {
			val["core"] = row.Core
		}
		out.Checks = append(out.Checks, CoreReindexCheck{
			Name: row.Name, Status: row.Status, Value: val, Note: row.Note,
		})
	}
}
