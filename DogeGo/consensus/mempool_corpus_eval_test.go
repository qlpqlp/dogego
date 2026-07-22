// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestEvalMempoolCorpus(t *testing.T) {
	rows, err := EvalMempoolCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 58 {
		t.Fatalf("corpus rows=%d want >=58", len(rows))
	}
	passed, failed, ok := SummarizeMempoolCorpusEval(rows)
	if !ok {
		for _, r := range rows {
			if !r.Match {
				t.Errorf("%s (%s): got accept=%v reason=%q want accept=%v reason=%q err=%q",
					r.Name, r.Template, r.GotAccept, r.GotRejectReason, r.WantAccept, r.WantRejectReason, r.Error)
			}
		}
		t.Fatalf("corpus eval failed: passed=%d failed=%d", passed, failed)
	}
}

// TestEvalMempoolCorpusStateful gates offline parity for templates that need a seeded mempool,
// UTXO view, or package graph (live reboottestnet probes mirror these via scripts).
func TestEvalMempoolCorpusStateful(t *testing.T) {
	rows, err := EvalMempoolCorpus()
	if err != nil {
		t.Fatal(err)
	}
	var stateful []MempoolCorpusEvalResult
	for _, r := range rows {
		if r.Stateful {
			stateful = append(stateful, r)
		}
	}
	if len(stateful) < 20 {
		t.Fatalf("stateful rows=%d want >=20", len(stateful))
	}
	passed, failed, ok := SummarizeMempoolCorpusEval(stateful)
	if !ok {
		for _, r := range stateful {
			if !r.Match {
				t.Errorf("%s (%s): got accept=%v reason=%q want accept=%v reason=%q",
					r.Name, r.Template, r.GotAccept, r.GotRejectReason, r.WantAccept, r.WantRejectReason)
			}
		}
		t.Fatalf("stateful corpus failed: passed=%d failed=%d", passed, failed)
	}
	t.Logf("stateful mempool corpus: passed=%d", passed)
}
