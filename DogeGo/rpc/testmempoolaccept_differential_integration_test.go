// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/wire"
)

// TestExecTestMempoolAcceptDifferentialVectors runs the shared mempool policy corpus through testmempoolaccept admission.
func TestExecTestMempoolAcceptDifferentialVectors(t *testing.T) {
	vecs, err := consensus.LoadMempoolDifferentialVectors()
	if err != nil {
		t.Fatal(err)
	}
	policyOnly := map[string]bool{
		"package_ancestor_limit": true, "package_descendant_limit": true, "package_ancestor_size": true, "package_descendant_size": true,
		"min_relay_fee": true, "rbf_insufficient_fee": true, "rbf_sufficient_fee": true, "rbf_not_replaceable": true, "rbf_fullrbf": true, "coinbase_immature": true,
		"rbf_too_many_descendants": true, "rbf_too_many_conflicts": true, "rbf_new_unconfirmed_input": true, "non_bip68_final": true,
	}
	for _, v := range vecs {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			fix, err := consensus.BuildMempoolDifferentialFixture(v.Template)
			if err != nil {
				t.Fatal(err)
			}
			pool := mempool.New(500)
			if fix.Prep != nil {
				if err := fix.Prep(pool); err != nil {
					t.Fatal(err)
				}
			}
			paths := mempoolAcceptPathsForTemplate(v.Template, fix)
			net := consensus.FixtureNetwork(fix.Net)
			var row map[string]interface{}
			if policyOnly[v.Template] {
				row = testMempoolAcceptPolicyRow(pool, paths, fix.Raw, net, v.Template)
			} else {
				hexParam, err := json.Marshal([]string{hex.EncodeToString(fix.Raw)})
				if err != nil {
					t.Fatal(err)
				}
				res, code, msg := execTestMempoolAccept(pool, nil, nil, nil, paths, []json.RawMessage{hexParam}, false, net)
				if code != 0 {
					t.Fatalf("rpc error code=%d msg=%q", code, msg)
				}
				rows, ok := res.([]map[string]interface{})
				if !ok || len(rows) != 1 {
					t.Fatalf("unexpected result type %T", res)
				}
				row = rows[0]
			}
			assertMempoolAcceptRow(t, row, v.WantAccept, v.WantRejectReason)
		})
	}
}

func assertMempoolAcceptRow(t *testing.T, row map[string]interface{}, wantAccept bool, wantReject string) {
	t.Helper()
	allowed, _ := row["allowed"].(bool)
	if allowed != wantAccept {
		reason, _ := row["reject-reason"].(string)
		t.Fatalf("allowed=%v want %v reject-reason=%q", allowed, wantAccept, reason)
	}
	if !wantAccept {
		got, _ := row["reject-reason"].(string)
		if got != wantReject && !strings.HasPrefix(got, wantReject) {
			t.Fatalf("reject-reason %q want %q", got, wantReject)
		}
	}
}

func mempoolAcceptPathsForTemplate(tmpl string, fix consensus.MempoolDifferentialFixture) *DataPaths {
	paths := &DataPaths{}
	switch tmpl {
	case "package_ancestor_size":
		lim := consensus.MempoolRelayLimits{LimitAncestorSizeKB: 1}
		paths.MempoolLimits = func() consensus.MempoolRelayLimits { return lim }
	case "package_descendant_size":
		lim := consensus.MempoolRelayLimits{LimitDescendantSizeKB: 1}
		paths.MempoolLimits = func() consensus.MempoolRelayLimits { return lim }
	case "rbf_fullrbf":
		paths.FullRBF = func() bool { return true }
	}
	if fix.View != nil {
		paths.MempoolAdmissionView = fix.View
	}
	if fix.Index != nil {
		paths.MempoolAdmissionIndex = fix.Index
	}
	if fix.Journal != nil {
		paths.MempoolAdmissionJournal = fix.Journal
	}
	if fix.Standard != nil {
		pol := *fix.Standard
		paths.Standard = func() consensus.StandardPolicy { return pol }
	}
	if paths.MempoolLimits == nil && paths.MempoolAdmissionView == nil && paths.MempoolAdmissionIndex == nil && paths.MempoolAdmissionJournal == nil && paths.Standard == nil && paths.FullRBF == nil {
		return nil
	}
	return paths
}

func testMempoolAcceptPolicyRow(pool *mempool.Pool, paths *DataPaths, raw []byte, net chain.Network, template string) map[string]interface{} {
	res := map[string]interface{}{"allowed": false}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		res["reject-reason"] = "TX decode failed"
		return res
	}
	adm := newMempoolAdmission(pool, nil, nil, nil, paths, net)
	if err := consensus.AcceptMempoolTxPolicy(tx, adm); err != nil {
		res["reject-reason"] = consensus.MempoolRejectReason(err)
		return res
	}
	res["allowed"] = true
	return res
}
