// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

// TestCoreMempoolVectorTemplatesCovered ensures every row in core_mempool_vectors.json has a harness handler.
func TestCoreMempoolVectorTemplatesCovered(t *testing.T) {
	vecs, err := LoadMempoolDifferentialVectors()
	if err != nil {
		t.Fatal(err)
	}
	handled := map[string]bool{
		"coinbase":                        true,
		"duplicate_vin":                   true,
		"missing_prevout":                 true,
		"min_relay_fee":                   true,
		"p2pkh_roundtrip":                 true,
		"p2sh_nested_p2pkh":               true,
		"p2sh_multisig":                   true,
		"bare_multisig":                   true,
		"p2sh_cltv_p2pk":                  true,
		"p2sh_csv_p2pk":                   true,
		"p2pk_non_standard_input":         true,
		"dust_output_reject":              true,
		"witness_reject":                  true,
		"bare_multisig_output_disabled":   true,
		"op_return_nonzero_reject":        true,
		"package_ancestor_limit":          true,
		"package_descendant_limit":        true,
		"package_ancestor_size":           true,
		"package_descendant_size":         true,
		"mempool_double_spend":            true,
		"rbf_insufficient_fee":            true,
		"rbf_sufficient_fee":              true,
		"rbf_not_replaceable":             true,
		"rbf_fullrbf":                     true,
		"coinbase_immature":               true,
		"vout_empty":                      true,
		"vout_negative":                   true,
		"vin_empty":                       true,
		"vout_toolarge":                   true,
		"prevout_null":                    true,
		"vout_empty_scriptpubkey":         true,
		"txouttotal_toolarge":             true,
		"tx_oversize":                     true,
		"unspendable_output":              true,
		"op_return_zero":                  true,
		"pq_commitment_op_return":         true,
		"pq_commitment_nonzero_reject":    true,
		"pq_carrier_p2sh_accept":          true,
		"absurd_fee":                      true,
		"multi_op_return":                 true,
		"tx_version_nonstandard":          true,
		"scriptsig_not_pushonly":          true,
		"non_final":                       true,
		"tx_size_small_reject":            true,
		"scriptsig_size_reject":           true,
		"discourage_nop_reject":           true,
		"op_return_oversize_reject":       true,
		"p2sh_sigops_reject":              true,
		"non_standard_output_reject":      true,
		"datacarrier_disabled_reject":     true,
		"p2sh_redeem_missing_reject":      true,
		"discourage_nop1_reject":          true,
		"rbf_too_many_descendants":      true,
		"rbf_too_many_conflicts":        true,
		"rbf_new_unconfirmed_input":     true,
		"tx_version_zero_reject":          true,
		"discourage_nop6_reject":          true,
		"non_bip68_final":               true,
	}
	seen := map[string]int{}
	for _, v := range vecs {
		if !handled[v.Template] {
			t.Fatalf("vector %q uses uncovered template %q", v.Name, v.Template)
		}
		seen[v.Template]++
	}
	if len(vecs) < len(handled) {
		t.Fatalf("corpus has %d rows, want at least %d (one per template)", len(vecs), len(handled))
	}
}
