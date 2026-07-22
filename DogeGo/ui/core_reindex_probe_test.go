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

func TestProbeCoreReindexMissingMethod(t *testing.T) {
	out := ProbeCoreReindex("mainnet", config.File{}, func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getrpcinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"method": map[string]interface{}{"getindexinfo": "help"},
			}}
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"blocks": float64(100), "initialblockdownload": false,
			}}
		case "getindexinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"txindex": map[string]interface{}{"synced": true},
			}}
		default:
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected"}}
		}
	})
	if out.OK {
		t.Fatalf("expected issues: %+v", out.Issues)
	}
}

func TestProbeCoreReindexOk(t *testing.T) {
	methods := map[string]interface{}{}
	for _, m := range coreReindexRequiredMethods {
		methods[m] = "help"
	}
	out := ProbeCoreReindex("testnet", config.File{}, func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getrpcinfo":
			return map[string]interface{}{"result": map[string]interface{}{"method": methods}}
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"blocks": float64(10), "initialblockdownload": true,
			}}
		case "getindexinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"txindex": map[string]interface{}{"synced": false},
				"basic":   map[string]interface{}{"synced": false},
			}}
		default:
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected"}}
		}
	})
	if !out.OK {
		t.Fatalf("expected ok: issues=%v warnings=%v", out.Issues, out.Warnings)
	}
	for _, w := range out.Warnings {
		if w == "block_filter_index_not_synced" {
			t.Fatal("expected no block filter warning during IBD")
		}
	}
}

func TestProbeCoreReindexIndexLagDuringCatchUp(t *testing.T) {
	methods := map[string]interface{}{}
	for _, m := range coreReindexRequiredMethods {
		methods[m] = "help"
	}
	out := ProbeCoreReindex("testnet", config.File{}, func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getrpcinfo":
			return map[string]interface{}{"result": map[string]interface{}{"method": methods}}
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"blocks": float64(50), "headers": float64(200), "initialblockdownload": false,
			}}
		case "getindexinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"txindex": map[string]interface{}{"synced": false},
				"basic":   map[string]interface{}{"synced": false},
			}}
		default:
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected"}}
		}
	})
	if !out.OK {
		t.Fatalf("expected ok during catch-up: issues=%v warnings=%v", out.Issues, out.Warnings)
	}
}

func TestProbeCoreReindexIndexCoreCompare(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result any
		switch req.Method {
		case "getblockchaininfo":
			result = map[string]any{"chain": "test", "blocks": float64(100)}
		case "getindexinfo":
			result = map[string]any{
				"txindex": map[string]any{"synced": true, "best_block_height": float64(100)},
				"basic":   map[string]any{"synced": true, "best_block_height": float64(100)},
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "id": 1})
	}))
	defer core.Close()
	coreAddr := strings.TrimPrefix(core.URL, "http://")

	methods := map[string]interface{}{}
	for _, m := range coreReindexRequiredMethods {
		methods[m] = "help"
	}
	out := ProbeCoreReindex("testnet", config.File{CoreRPCAddr: coreAddr}, func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getrpcinfo":
			return map[string]interface{}{"result": map[string]interface{}{"method": methods}}
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"blocks": float64(100), "headers": float64(100), "initialblockdownload": false,
			}}
		case "getindexinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"txindex":            map[string]interface{}{"synced": true, "best_block_height": float64(100)},
				"basic block filter": map[string]interface{}{"synced": true, "best_block_height": float64(100)},
			}}
		default:
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected"}}
		}
	})
	if !out.CoreAvailable {
		t.Fatalf("expected core available: %+v", out)
	}
	names := map[string]bool{}
	for _, c := range out.Checks {
		names[c.Name] = true
	}
	for _, want := range []string{"getindexinfo.txindex.synced", "getindexinfo.basic.synced"} {
		if !names[want] {
			t.Fatalf("missing check %q: %+v", want, out.Checks)
		}
	}
}

func TestConnectLagFromInfo(t *testing.T) {
	lag, ok := connectLagFromInfo(map[string]any{
		"dogego_connect_lag": float64(42),
	})
	if !ok || lag != 42 {
		t.Fatalf("got %d %v", lag, ok)
	}
	lag, ok = connectLagFromInfo(map[string]any{
		"blocks": float64(100), "dogego_contiguous_raw_height": float64(250),
	})
	if !ok || lag != 150 {
		t.Fatalf("derived lag %d %v", lag, ok)
	}
}

func TestAppendConnectLagCheckIBD(t *testing.T) {
	out := &CoreRestartResumeResult{IBD: true}
	appendConnectLagCheck(out, map[string]any{"dogego_connect_lag": float64(4096)})
	if out.ConnectLag != 4096 {
		t.Fatalf("connect lag %d", out.ConnectLag)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings %+v", out.Warnings)
	}
}

func TestAppendConnectLagCheckPopulatesBoostFields(t *testing.T) {
	out := &CoreRestartResumeResult{IBD: true}
	appendConnectLagCheck(out, map[string]any{
		"dogego_connect_lag":                  float64(42),
		"dogego_connect_catch_up_passes":      float64(8),
		"dogego_connect_catch_up_batch":       float64(128),
		"dogego_connect_catch_up_interval_ms": float64(500),
	})
	if out.ConnectCatchUpPasses != 8 || out.ConnectCatchUpBatch != 128 || out.ConnectCatchUpIntervalMs != 500 {
		t.Fatalf("boost fields passes=%d batch=%d ms=%d", out.ConnectCatchUpPasses, out.ConnectCatchUpBatch, out.ConnectCatchUpIntervalMs)
	}
	found := false
	for _, c := range out.Checks {
		if c.Name == "connect_lag" && strings.Contains(c.Note, "boost") {
			found = true
		}
	}
	if !found {
		t.Fatalf("checks=%+v", out.Checks)
	}
}

func TestConnectLagCheckNoteBoost(t *testing.T) {
	note := connectLagCheckNote(map[string]any{
		"dogego_connect_catch_up_passes":      float64(8),
		"dogego_connect_catch_up_batch":       float64(128),
		"dogego_connect_catch_up_interval_ms": float64(500),
	}, 128, true)
	if note == "" || note == "above 128 during IBD" {
		t.Fatalf("note=%q", note)
	}
}
