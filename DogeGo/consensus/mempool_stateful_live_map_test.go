// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatefulMempoolLiveMapCoversKeyTemplates(t *testing.T) {
	want := []string{
		"dust_output_reject", "absurd_fee", "non_final", "rbf_insufficient_fee", "rbf_sufficient_fee",
		"coinbase_immature", "package_ancestor_limit", "min_relay_fee", "mempool_double_spend",
		"rbf_not_replaceable", "rbf_fullrbf", "package_descendant_limit",
		"p2pkh_roundtrip", "rbf_too_many_descendants",
		"p2sh_nested_p2pkh", "p2sh_multisig", "bare_multisig", "p2sh_cltv_p2pk", "p2sh_csv_p2pk",
		"p2pk_non_standard_input", "package_ancestor_size", "package_descendant_size",
		"pq_commitment_op_return", "pq_carrier_p2sh_accept",
	}
	for _, tmpl := range want {
		if _, ok := StatefulMempoolLiveScenario(tmpl); !ok {
			t.Fatalf("template %q missing live scenario mapping", tmpl)
		}
	}
	if n := len(StatefulMempoolLiveScenarios()); n < 24 {
		t.Fatalf("live scenarios=%d want >=24", n)
	}
}

func TestStatefulMempoolLiveScriptDocumentsScenarios(t *testing.T) {
	path := filepath.Join(repoRootForConsensus(t), "scripts", "mempool_stateful_parity_reboottestnet.ps1")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, s := range StatefulMempoolLiveScenarios() {
		if !strings.Contains(text, `"`+s+`"`) {
			t.Fatalf("mempool_stateful script missing scenario %q", s)
		}
	}
}

func TestStatefulMempoolCoverageSummary(t *testing.T) {
	rows, err := EvalMempoolCorpus()
	if err != nil {
		t.Fatal(err)
	}
	var stateful, liveMapped int
	for _, r := range rows {
		if !r.Stateful {
			continue
		}
		stateful++
		if _, ok := StatefulMempoolLiveScenario(r.Template); ok {
			liveMapped++
		}
	}
	if stateful != 26 {
		t.Fatalf("stateful=%d want 26 (24 live-mapped + 2 offline-only BIP125 rule 2/5)", stateful)
	}
	if liveMapped != 24 {
		t.Fatalf("live-mapped=%d want 24 (full live wallet-anchored coverage)", liveMapped)
	}
	t.Logf("stateful=%d live=%d offline-only=%d", stateful, liveMapped, stateful-liveMapped)
}

func repoRootForConsensus(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
