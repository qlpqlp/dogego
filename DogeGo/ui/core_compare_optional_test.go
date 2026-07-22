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

func TestFloatNearlyEqual(t *testing.T) {
	if !floatNearlyEqual(1.0, 1.0) {
		t.Fatal("equal")
	}
	if !floatNearlyEqual(12345.0, 12345.0000001) {
		t.Fatal("near equal")
	}
	if floatNearlyEqual(1.0, 2.0) {
		t.Fatal("not equal")
	}
}

func TestProbeCoreCompareSideBySideExtended(t *testing.T) {
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result any
		switch req.Method {
		case "getblockchaininfo":
			result = map[string]interface{}{
				"chain": "test", "blocks": float64(100), "headers": float64(100),
				"difficulty": 12345.0, "initialblockdownload": false,
				"bestblockhash": "abc", "chainwork": "000abc", "mediantime": float64(1_700_000_000),
				"verificationprogress": 0.9999,
				"softforks": []interface{}{
					map[string]interface{}{"id": "bip34", "reject": map[string]interface{}{"status": true}},
					map[string]interface{}{"id": "bip66", "reject": map[string]interface{}{"status": true}},
					map[string]interface{}{"id": "bip65", "reject": map[string]interface{}{"status": true}},
				},
				"bip9_softforks": map[string]interface{}{
					"csv": map[string]interface{}{"status": "active"},
				},
			}
		case "verifychain":
			result = true
		case "gettxoutsetinfo":
			result = map[string]interface{}{"height": float64(100), "hash_serialized": "abc"}
		case "getmempoolinfo":
			result = map[string]interface{}{
				"size": float64(2), "fullrbf": false,
				"minrelaytxfee": 0.001, "incrementalrelayfee": 0.0001,
			}
		case "getnetworkinfo":
			result = map[string]interface{}{"version": float64(170100)}
		case "getdeploymentinfo":
			result = map[string]interface{}{
				"deployments": map[string]interface{}{
					"bip34": map[string]interface{}{"type": "buried", "active": true, "height": float64(21111)},
					"bip66": map[string]interface{}{"type": "buried", "active": true, "height": float64(21111)},
					"csv":   map[string]interface{}{"type": "bip9", "active": true, "height": float64(770112), "bip9": map[string]interface{}{"bit": float64(0), "status": "active", "since": float64(770112)}},
				},
			}
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
				"difficulty": 12345.0, "initialblockdownload": false,
				"dogego_connect_lag": float64(0),
				"bestblockhash": "abc", "chainwork": "000abc", "mediantime": float64(1_700_000_000),
				"verificationprogress": 0.9999,
				"softforks": []interface{}{
					map[string]interface{}{"id": "bip34", "reject": map[string]interface{}{"status": true}},
					map[string]interface{}{"id": "bip66", "reject": map[string]interface{}{"status": true}},
					map[string]interface{}{"id": "bip65", "reject": map[string]interface{}{"status": true}},
				},
				"bip9_softforks": map[string]interface{}{
					"csv": map[string]interface{}{"status": "active"},
				},
			}}
		case "verifychain":
			return map[string]interface{}{"result": true}
		case "getchaintips":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{"status": "active"},
			}}
		case "gettxoutsetinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"height": float64(100), "hash_serialized": "abc",
			}}
		case "getmempoolinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"size": float64(2), "fullrbf": false,
				"minrelaytxfee": 0.001, "incrementalrelayfee": 0.0001,
				"dogego_package_policy": map[string]interface{}{
					"limitancestorcount": 25, "limitdescendantcount": 25,
				},
			}}
		case "getnetworkinfo":
			return map[string]interface{}{"result": map[string]interface{}{"version": float64(170100)}}
		case "getdeploymentinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"deployments": map[string]interface{}{
					"bip34": map[string]interface{}{"type": "buried", "active": true, "height": float64(21111)},
					"bip66": map[string]interface{}{"type": "buried", "active": true, "height": float64(21111)},
					"csv":   map[string]interface{}{"type": "bip9", "active": true, "height": float64(770112), "bip9": map[string]interface{}{"bit": float64(0), "status": "active", "since": float64(770112)}},
				},
			}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	out := ProbeCoreCompare("testnet", "127.0.0.1:44555", config.File{CoreRPCAddr: coreAddr}, invoke)
	if !out.CoreConfigured || !out.Available {
		t.Fatalf("expected side-by-side compare available: configured=%v available=%v errors=%v", out.CoreConfigured, out.Available, out.Errors)
	}
	names := make(map[string]bool)
	for _, f := range out.Fields {
		names[f.Name] = true
	}
	for _, want := range []string{
		"difficulty", "dogego_connect_lag", "getmempoolinfo.size", "getmempoolinfo.fullrbf",
		"getmempoolinfo.minrelaytxfee", "getmempoolinfo.incrementalrelayfee",
		"getmempoolinfo.dogego_package_policy", "getnetworkinfo.version",
		"gettxoutsetinfo.height",
		"chainwork", "mediantime", "verificationprogress",
		"deployment.bip34.active", "deployment.csv.active", "deployment.protocol_lock",
		"deployment.csv.bit", "deployment.csv.status", "deployment.csv.since",
		"deployment.bip34.height",
		"softfork.bip34.reject", "bip9_softfork.csv.active",
	} {
		if !names[want] {
			t.Fatalf("missing field %q: %+v", want, out.Fields)
		}
	}
	for _, f := range out.Fields {
		if f.Name == "deployment.protocol_lock" && !f.Match {
			t.Fatalf("expected protocol lock match, got %+v", f)
		}
	}
	if !out.ConnectLagOK {
		t.Fatal("expected connect lag ok")
	}
}

func TestDeploymentActiveMap(t *testing.T) {
	m := deploymentActiveMap(map[string]interface{}{
		"bip34": map[string]interface{}{"active": true},
		"bip66": map[string]interface{}{"active": false},
		"csv":   map[string]interface{}{"active": true},
	})
	if len(m) != 3 || !m["bip34"] || m["bip66"] || !m["csv"] {
		t.Fatalf("unexpected active map: %+v", m)
	}
	if deploymentActiveMap("not-a-map") != nil {
		t.Fatal("expected nil for non-map input")
	}
}

func TestDeploymentActiveMapDetectsMismatch(t *testing.T) {
	dg := deploymentActiveMap(map[string]interface{}{"csv": map[string]interface{}{"active": true}})
	core := deploymentActiveMap(map[string]interface{}{"csv": map[string]interface{}{"active": false}})
	if dg["csv"] == core["csv"] {
		t.Fatal("expected differing active-state between nodes")
	}
}

func TestProbeCoreCompareOptionalWithoutCoreConfig(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		if method == "getblockchaininfo" {
			return map[string]interface{}{"result": map[string]interface{}{"chain": "test", "blocks": float64(1), "headers": float64(1)}}
		}
		return map[string]interface{}{"result": nil}
	}
	out := ProbeCoreCompare("testnet", "127.0.0.1:44555", config.File{}, invoke)
	if out.CoreConfigured {
		t.Fatal("expected Core not explicitly configured")
	}
	if out.Available {
		t.Fatal("expected Core unavailable without config")
	}
}

func TestApplyCoreOperatorCertCompareOptional(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		Compare: CoreCompareResult{Available: false, CoreConfigured: false},
	})
	var cmp *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "core_compare" {
			cmp = &rows[i]
			break
		}
	}
	if cmp == nil || cmp.OK == nil || !*cmp.OK {
		t.Fatalf("expected optional compare OK: %+v", cmp)
	}
}
