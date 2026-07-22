// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dogego/config"
)

func TestIndexSection(t *testing.T) {
	idx := map[string]any{
		"basic block filter": map[string]any{"synced": true},
	}
	if got := indexSection(idx, "basic", "basic block filter"); got == nil || !boolFromAny(got["synced"]) {
		t.Fatalf("indexSection=%#v", got)
	}
}

func TestCompareIndexInfoWithCoreMatch(t *testing.T) {
	dg := map[string]any{
		"txindex": map[string]any{"synced": true, "best_block_height": float64(100)},
		"basic block filter": map[string]any{"synced": true, "best_block_height": float64(100)},
		"coinstatsindex": map[string]any{"synced": true, "hash_serialized": "abc"},
	}
	core := map[string]any{
		"txindex": map[string]any{"synced": true, "best_block_height": float64(100)},
		"basic":   map[string]any{"synced": true, "best_block_height": float64(100)},
		"coinstatsindex": map[string]any{"synced": true, "hash_serialized": "abc"},
	}
	out := &CoreMaintenanceResult{}
	compareIndexInfoWithCore(out, dg, core)
	if len(out.Warnings) != 0 {
		t.Fatalf("warnings=%v checks=%+v", out.Warnings, out.Checks)
	}
	names := map[string]bool{}
	for _, c := range out.Checks {
		names[c.Name] = true
	}
	for _, want := range []string{
		"getindexinfo.txindex.synced",
		"getindexinfo.txindex.best_block_height",
		"getindexinfo.basic.synced",
		"getindexinfo.coinstatsindex.hash_serialized",
	} {
		if !names[want] {
			t.Fatalf("missing check %q: %+v", want, out.Checks)
		}
	}
}

func TestCompareIndexInfoWithCoreSyncedMismatch(t *testing.T) {
	dg := map[string]any{
		"txindex": map[string]any{"synced": true, "best_block_height": float64(100)},
	}
	core := map[string]any{
		"txindex": map[string]any{"synced": false, "best_block_height": float64(100)},
	}
	out := &CoreMaintenanceResult{}
	compareIndexInfoWithCore(out, dg, core)
	if len(out.Warnings) == 0 {
		t.Fatal("expected synced mismatch warning")
	}
}

func TestProbeCoreMaintenanceIndexCoreCompare(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result any
		switch req.Method {
		case "getblockchaininfo":
			result = map[string]any{
				"chain": "test", "blocks": float64(100), "headers": float64(100),
				"initialblockdownload": false, "bestblockhash": "abc",
			}
		case "getindexinfo":
			result = map[string]any{
				"txindex": map[string]any{"synced": true, "best_block_height": float64(100)},
				"basic":   map[string]any{"synced": true, "best_block_height": float64(100)},
			}
		case "getchaintxstats":
			result = map[string]any{"window_tx_count": float64(50)}
		case "verifychain":
			result = true
		case "getblockfilter":
			result = map[string]any{"filter": "00"}
		default:
			result = nil
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "id": 1})
	}))
	defer core.Close()
	coreAddr := strings.TrimPrefix(core.URL, "http://")

	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"chain": "test", "blocks": float64(100), "headers": float64(100),
				"initialblockdownload": false,
			}}
		case "verifychain":
			return map[string]interface{}{"result": true}
		case "getindexinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"txindex":            map[string]interface{}{"synced": true, "best_block_height": float64(100)},
				"basic block filter": map[string]interface{}{"synced": true, "best_block_height": float64(100)},
			}}
		case "getchaintxstats":
			return map[string]interface{}{"result": map[string]interface{}{"window_tx_count": float64(50), "time": float64(1_700_000_000)}}
		case "getbestblockhash":
			return map[string]interface{}{"result": "abc"}
		case "getblockfilter":
			return map[string]interface{}{"result": map[string]interface{}{"filter": "00"}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	out := ProbeCoreMaintenance("testnet", config.File{CoreRPCAddr: coreAddr}, invoke)
	if !out.CoreAvailable {
		t.Fatalf("expected core available: %+v", out)
	}
	names := map[string]bool{}
	for _, c := range out.Checks {
		names[c.Name] = true
	}
	for _, want := range []string{
		"verifychain(2,0)",
		"verifychain(4,0)",
		"verifychain(2,0).core_compare",
		"verifychain(4,0).core_compare",
		"getblockfilter(tip).core_compare",
		"getindexinfo.txindex.synced",
		"getindexinfo.basic.synced",
		"chaintxstats_window",
	} {
		if !names[want] {
			t.Fatalf("missing check %q: %+v", want, out.Checks)
		}
	}
}
