// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"testing"
)

func TestAppendDogegoVerifyChainLevel2FailWarning(t *testing.T) {
	invoke := func(method string, params []json.RawMessage) map[string]interface{} {
		if method != "verifychain" {
			return map[string]interface{}{"result": nil}
		}
		var p []any
		_ = json.Unmarshal(params[0], &p)
		if len(p) > 0 {
			if lvl, ok := p[0].(float64); ok && int(lvl) == 2 {
				return map[string]interface{}{"result": false}
			}
		}
		return map[string]interface{}{"result": true}
	}
	out := CoreMaintenanceResult{}
	probeMaintenanceVerifyChain(&out, CoreParityEndpoints{}, false, invoke, false)
	if len(out.Issues) != 0 {
		t.Fatalf("level 2 false should warn not fail issues: %v", out.Issues)
	}
	found := false
	for _, w := range out.Warnings {
		if w == "verifychain_2_not_true" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings=%v", out.Warnings)
	}
}

func TestAppendDogegoVerifyChainLevels(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		if method == "verifychain" {
			return map[string]interface{}{"result": true}
		}
		return map[string]interface{}{"result": nil}
	}
	out := CoreMaintenanceResult{}
	probeMaintenanceVerifyChain(&out, CoreParityEndpoints{}, false, invoke, false)
	names := map[string]bool{}
	for _, c := range out.Checks {
		names[c.Name] = true
	}
	for _, want := range []string{"verifychain(2,0)", "verifychain(4,0)"} {
		if !names[want] {
			t.Fatalf("missing %q: %+v", want, out.Checks)
		}
	}
}
