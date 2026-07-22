// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"testing"

	"dogego/config"
)

func TestExpectedDeploymentsActiveTestnetGenesis(t *testing.T) {
	exp, err := expectedDeploymentsActive("testnet", 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bip34", "bip66", "bip65", "csv"} {
		if exp[name] {
			t.Fatalf("%s should be inactive at height 1", name)
		}
	}
}

func TestExpectedDeploymentsActiveMainnetTip(t *testing.T) {
	exp, err := expectedDeploymentsActive("mainnet", 5_000_000)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bip34", "bip66", "bip65", "csv"} {
		if !exp[name] {
			t.Fatalf("%s should be active at mainnet height 5M", name)
		}
	}
}

func TestProbeCoreCompareSoloDeploymentSanity(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"chain": "test", "blocks": float64(1), "headers": float64(1),
				"initialblockdownload": true,
				"softforks": []interface{}{
					map[string]interface{}{"id": "bip34", "reject": map[string]interface{}{"status": false}},
					map[string]interface{}{"id": "bip66", "reject": map[string]interface{}{"status": false}},
					map[string]interface{}{"id": "bip65", "reject": map[string]interface{}{"status": false}},
				},
				"bip9_softforks": map[string]interface{}{
					"csv": map[string]interface{}{"status": "defined"},
				},
			}}
		case "getdeploymentinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"height": float64(1),
				"deployments": map[string]interface{}{
					"bip34": map[string]interface{}{"active": false},
					"bip66": map[string]interface{}{"active": false},
					"bip65": map[string]interface{}{"active": false},
					"csv":   map[string]interface{}{"active": false},
				},
			}}
		case "verifychain":
			return map[string]interface{}{"result": true}
		case "getchaintips":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{"status": "active"},
			}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	out := ProbeCoreCompare("testnet", "127.0.0.1:44555", config.File{}, invoke)
	if out.CoreConfigured || out.Available {
		t.Fatalf("expected solo-only compare: configured=%v available=%v", out.CoreConfigured, out.Available)
	}
	if !out.DeploymentChecked || !out.ProtocolLockOK {
		t.Fatalf("expected solo deployment sanity pass: checked=%v lock=%v fields=%+v", out.DeploymentChecked, out.ProtocolLockOK, out.Fields)
	}
}

func TestProbeCoreCompareSoloDeploymentMismatchFails(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"chain": "main", "blocks": float64(5_000_000), "headers": float64(5_000_000),
				"initialblockdownload": false,
				"softforks": []interface{}{
					map[string]interface{}{"id": "bip34", "reject": map[string]interface{}{"status": true}},
					map[string]interface{}{"id": "bip66", "reject": map[string]interface{}{"status": true}},
					map[string]interface{}{"id": "bip65", "reject": map[string]interface{}{"status": true}},
				},
				"bip9_softforks": map[string]interface{}{
					"csv": map[string]interface{}{"status": "active"},
				},
			}}
		case "getdeploymentinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"height": float64(5_000_000),
				"deployments": map[string]interface{}{
					"bip34": map[string]interface{}{"active": true},
					"bip66": map[string]interface{}{"active": true},
					"bip65": map[string]interface{}{"active": true},
					"csv":   map[string]interface{}{"active": false},
				},
			}}
		case "verifychain":
			return map[string]interface{}{"result": true}
		case "getchaintips":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{"status": "active"},
			}}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	out := ProbeCoreCompare("mainnet", "127.0.0.1:22557", config.File{}, invoke)
	if !out.DeploymentChecked || out.ProtocolLockOK {
		t.Fatalf("expected protocol lock failure: checked=%v lock=%v", out.DeploymentChecked, out.ProtocolLockOK)
	}
}

func TestCompareDeploymentDetailMatch(t *testing.T) {
	dg := map[string]interface{}{
		"csv": map[string]interface{}{"type": "bip9", "active": true, "height": float64(100), "bip9": map[string]interface{}{"bit": float64(0), "status": "active", "since": float64(100)}},
	}
	core := map[string]interface{}{
		"csv": map[string]interface{}{"type": "bip9", "active": true, "height": float64(100), "bip9": map[string]interface{}{"bit": float64(0), "status": "active", "since": float64(100)}},
	}
	out := &CoreCompareResult{}
	if !compareDeploymentDetail(out, dg, core) {
		t.Fatalf("expected match, fields=%+v", out.Fields)
	}
	names := map[string]bool{}
	for _, f := range out.Fields {
		names[f.Name] = true
	}
	for _, want := range []string{"deployment.csv.bit", "deployment.csv.status", "deployment.csv.since", "deployment.csv.height"} {
		if !names[want] {
			t.Fatalf("missing field %q", want)
		}
	}
}

func TestCompareDeploymentDetailStatusMismatch(t *testing.T) {
	dg := map[string]interface{}{
		"csv": map[string]interface{}{"type": "bip9", "active": true, "bip9": map[string]interface{}{"bit": float64(0), "status": "active", "since": float64(100)}},
	}
	core := map[string]interface{}{
		"csv": map[string]interface{}{"type": "bip9", "active": false, "bip9": map[string]interface{}{"bit": float64(0), "status": "started", "since": float64(50)}},
	}
	out := &CoreCompareResult{}
	if compareDeploymentDetail(out, dg, core) {
		t.Fatal("expected status mismatch to fail")
	}
}

func TestCompareDeploymentDetailBitMismatch(t *testing.T) {
	dg := map[string]interface{}{
		"csv": map[string]interface{}{"type": "bip9", "bip9": map[string]interface{}{"bit": float64(0), "status": "defined"}},
	}
	core := map[string]interface{}{
		"csv": map[string]interface{}{"type": "bip9", "bip9": map[string]interface{}{"bit": float64(1), "status": "defined"}},
	}
	out := &CoreCompareResult{}
	if compareDeploymentDetail(out, dg, core) {
		t.Fatal("expected bit mismatch to fail")
	}
}

func TestCompareSoftforkParityMatch(t *testing.T) {
	dg := map[string]any{
		"softforks": []any{
			map[string]any{"id": "bip34", "reject": map[string]any{"status": true}},
		},
		"bip9_softforks": map[string]any{
			"csv": map[string]any{"status": "active"},
		},
	}
	core := map[string]any{
		"softforks": []any{
			map[string]any{"id": "bip34", "reject": map[string]any{"status": true}},
		},
		"bip9_softforks": map[string]any{
			"csv": map[string]any{"status": "active"},
		},
	}
	out := &CoreCompareResult{}
	if !compareSoftforkParity(out, dg, core) {
		t.Fatalf("expected match, fields=%+v", out.Fields)
	}
}

func TestCompareSoftforkSoloSanityMismatch(t *testing.T) {
	dg := map[string]any{
		"blocks": float64(5_000_000),
		"softforks": []any{
			map[string]any{"id": "bip34", "reject": map[string]any{"status": false}},
		},
	}
	out := &CoreCompareResult{}
	if compareSoftforkSoloSanity(out, dg, "mainnet") {
		t.Fatal("expected bip34 reject mismatch at mainnet tip")
	}
	if !out.DeploymentChecked {
		t.Fatal("expected deployment checked from softfork solo")
	}
}

func TestApplyCoreOperatorCertSoloProtocolLock(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		Compare: CoreCompareResult{
			Available:         false,
			CoreConfigured:    false,
			DeploymentChecked: true,
			ProtocolLockOK:    true,
		},
	})
	var cmp *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "core_compare" {
			cmp = &rows[i]
			break
		}
	}
	if cmp == nil || cmp.OK == nil || !*cmp.OK {
		t.Fatalf("expected solo protocol lock OK: %+v", cmp)
	}
}
