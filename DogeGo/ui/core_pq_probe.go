// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"dogego/consensus"
	"dogego/pqcrypto"
	"dogego/wire"
)

// CorePQProbeCheck is one PQ format/carrier verification row.
type CorePQProbeCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warning, issue
	Value  any    `json:"value,omitempty"`
	Note   string `json:"note,omitempty"`
}

// CorePQProbeResult is returned by GET /api/core-pq-probe (Features tab PQ cert gate).
type CorePQProbeResult struct {
	CheckedAt string             `json:"checked_at"`
	OK        bool               `json:"ok"`
	Checks    []CorePQProbeCheck `json:"checks"`
	Notes     []string           `json:"notes,omitempty"`
	Issues    []string           `json:"issues,omitempty"`
	Hint      string             `json:"hint,omitempty"`
}

// ProbeCorePQ runs in-process OP_RETURN + TX_C/TX_R carrier format/crypto checks (no network).
func ProbeCorePQ() CorePQProbeResult {
	out := CorePQProbeResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		OK:        true,
		Hint:      "Verifier-side PQ MVP: OP_RETURN FLC1/DIL2/RCG4 format + TX_C/TX_R carrier round-trip via pqcrypto. Not consensus-enforced; not production PQ safety. Offline bundle: dogego cert pq.",
		Notes:     []string{"pq_probe_offline"},
	}
	probePQCommitmentTags(&out)
	probePQCarrierSchemes(&out)
	probePQMempoolCorpus(&out)
	out.OK = len(out.Issues) == 0
	return out
}

func probePQCommitmentTags(out *CorePQProbeResult) {
	tags := []struct {
		tag    string
		scheme string
	}{
		{consensus.PQTagFalcon, "falcon-512"},
		{consensus.PQTagDilithium, "dilithium2"},
		{consensus.PQTagRaccoon, "raccoon-g-44"},
	}
	for _, row := range tags {
		commit := sha256.Sum256([]byte("dogego/pq/probe/" + row.tag))
		script, err := consensus.BuildPQCommitmentScript(row.tag, commit[:])
		if err != nil {
			out.Issues = append(out.Issues, "commitment_build_"+row.tag)
			out.Checks = append(out.Checks, CorePQProbeCheck{
				Name: "op_return_" + row.tag, Status: "issue", Note: err.Error(),
			})
			continue
		}
		verified, err := consensus.VerifyPQCommitmentScriptHex(hex.EncodeToString(script))
		if err != nil || verified["valid"] != true {
			out.Issues = append(out.Issues, "commitment_verify_"+row.tag)
			note := "verify failed"
			if err != nil {
				note = err.Error()
			}
			out.Checks = append(out.Checks, CorePQProbeCheck{
				Name: "op_return_" + row.tag, Status: "issue", Note: note,
			})
			continue
		}
		out.Checks = append(out.Checks, CorePQProbeCheck{
			Name: "op_return_" + row.tag, Status: "ok", Value: row.scheme,
			Note: "Phase-1 OP_RETURN format",
		})
	}
}

func probePQCarrierSchemes(out *CorePQProbeResult) {
	schemes := []struct {
		scheme pqcrypto.Scheme
		tag    string
		label  string
	}{
		{pqcrypto.Falcon512{}, consensus.PQTagFalcon, "falcon"},
		{pqcrypto.Dilithium2{}, consensus.PQTagDilithium, "dilithium"},
		{pqcrypto.RaccoonG44{}, consensus.PQTagRaccoon, "raccoon"},
	}
	pkScript := pqProbeCarrierSpendScript()
	for _, row := range schemes {
		ok, note := pqProbeCarrierRoundTrip(row.scheme, row.label, pkScript)
		st := "ok"
		if !ok {
			st = "issue"
			out.Issues = append(out.Issues, "carrier_"+row.tag)
		}
		check := CorePQProbeCheck{
			Name: "carrier_" + row.tag, Status: st, Value: row.scheme.Name(), Note: note,
		}
		if row.tag == consensus.PQTagRaccoon {
			check.Note = note + " (experimental deterministic backend; not libdogecoin-compatible)"
			out.Notes = append(out.Notes, "raccoon_experimental_backend")
		}
		out.Checks = append(out.Checks, check)
	}
}

func probePQMempoolCorpus(out *CorePQProbeResult) {
	vecs, err := consensus.LoadMempoolDifferentialVectors()
	if err != nil || len(vecs) == 0 {
		out.Checks = append(out.Checks, CorePQProbeCheck{
			Name: "mempool_corpus_pq", Status: "warning",
			Note: "mempool differential vectors unavailable",
		})
		return
	}
	want := map[string]bool{
		"pq_commitment_op_return":      false,
		"pq_commitment_nonzero_reject": false,
		"pq_carrier_p2sh_accept":       false,
	}
	for _, v := range vecs {
		if _, ok := want[v.Template]; ok {
			want[v.Template] = true
		}
	}
	missing := make([]string, 0)
	for tpl, found := range want {
		if !found {
			missing = append(missing, tpl)
		}
	}
	if len(missing) > 0 {
		out.Issues = append(out.Issues, "mempool_corpus_pq_missing")
		out.Checks = append(out.Checks, CorePQProbeCheck{
			Name: "mempool_corpus_pq", Status: "issue",
			Note: "missing templates: " + fmt.Sprint(missing),
		})
		return
	}
	out.Checks = append(out.Checks, CorePQProbeCheck{
		Name: "mempool_corpus_pq", Status: "ok",
		Note: "pq_commitment_op_return + pq_carrier_p2sh_accept offline rows",
	})
}

func pqProbeCarrierSpendScript() []byte {
	return []byte{
		0x76, 0xa9, 0x14,
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		0x88, 0xac,
	}
}

func pqProbeCarrierFundedTx(pkScript []byte) *wire.Tx {
	return &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{},
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 10_000_000_000, PkScript: pkScript}},
	}
}

func pqProbeCarrierRoundTrip(scheme pqcrypto.Scheme, label string, pkScript []byte) (bool, string) {
	seed := pqcrypto.DeriveSeed([]byte("pq-probe/"+label), scheme.Name())
	pk, sk, err := scheme.GenerateKey(seed[:])
	if err != nil {
		return false, err.Error()
	}
	plan, err := consensus.BuildPQCarrierTransactions(
		pqProbeCarrierFundedTx(pkScript),
		scheme, pk, sk, 0, pkScript, wire.SigHashAll, consensus.PQCarrierMinOutputKoinu(),
	)
	if err != nil {
		return false, err.Error()
	}
	out, err := consensus.VerifyPQCarrierPair(plan.TXC, plan.TXR, 0, pkScript, wire.SigHashAll, nil)
	if err != nil {
		return false, err.Error()
	}
	if out["valid"] != true || out["pq_verify"] != "passed" {
		return false, fmt.Sprintf("valid=%v pq_verify=%v", out["valid"], out["pq_verify"])
	}
	return true, "TX_C/TX_R linkage + pqcrypto verify"
}
