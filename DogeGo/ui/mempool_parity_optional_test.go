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

func TestRunMempoolParityProbeNoCoreWhenUnconfigured(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		if method != "testmempoolaccept" {
			return map[string]interface{}{"result": nil}
		}
		return map[string]interface{}{
			"result": []interface{}{
				map[string]interface{}{"allowed": false, "reject-reason": "mock"},
			},
		}
	}
	out := RunMempoolParityProbe("testnet", config.File{}, invoke)
	if out.CoreConfigured {
		t.Fatal("expected Core compare disabled without explicit config")
	}
	if out.CoreAvailable {
		t.Fatal("expected Core unavailable when compare disabled")
	}
	if out.OfflineStateful == nil || out.OfflineStateful.Total < 10 {
		t.Fatalf("expected offline stateful summary: %+v", out.OfflineStateful)
	}
}

func TestApplyCoreOperatorCertMempoolOptional(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		MempoolParity: MempoolParityProbeResult{
			OK:             true,
			Total:          31,
			Passed:         31,
			CoreConfigured: false,
			OfflineStateful: &MempoolOfflineStatefulSummary{
				OK: true, Total: 24, Passed: 24,
			},
		},
	})
	var mp *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "mempool_parity" {
			mp = &rows[i]
			break
		}
	}
	if mp == nil || mp.OK == nil || !*mp.OK {
		t.Fatalf("expected mempool parity OK: %+v", mp)
	}
	if mp.Note == "" || mp.Note == "Core drift on one or more rows" {
		t.Fatalf("expected optional note: %q", mp.Note)
	}
}
