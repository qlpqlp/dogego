// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// StatefulMempoolLiveScenario names a reboottestnet live probe in
// scripts/mempool_stateful_parity_reboottestnet.ps1 (-Scenario).
const (
	LiveScenarioDust                   = "dust"
	LiveScenarioAbsurdFee              = "absurd_fee"
	LiveScenarioNonFinal               = "non_final"
	LiveScenarioRBFInsufficient        = "rbf_insufficient"
	LiveScenarioRBFSufficient          = "rbf_sufficient"
	LiveScenarioCoinbaseImmature         = "coinbase_immature"
	LiveScenarioPackageAncestorLimit   = "package_ancestor_limit"
	LiveScenarioMinRelayFee            = "min_relay_fee"
	LiveScenarioMempoolDoubleSpend     = "mempool_double_spend"
	LiveScenarioRBFNotReplaceable      = "rbf_not_replaceable"
	LiveScenarioRBFFullRBF             = "rbf_fullrbf"
	LiveScenarioPackageDescendantLimit = "package_descendant_limit"
	LiveScenarioP2PKHRoundtrip         = "p2pkh_roundtrip"
	LiveScenarioRBFTooManyDescendants  = "rbf_too_many_descendants"
	LiveScenarioP2SHNestedP2PKH        = "p2sh_nested_p2pkh"
	LiveScenarioP2SHMultisig           = "p2sh_multisig"
	LiveScenarioBareMultisig           = "bare_multisig"
	LiveScenarioP2SHCLTVP2PK           = "p2sh_cltv_p2pk"
	LiveScenarioP2SHCSVP2PK            = "p2sh_csv_p2pk"
	LiveScenarioP2PKNonStandardInput   = "p2pk_non_standard_input"
	LiveScenarioPackageAncestorSize    = "package_ancestor_size"
	LiveScenarioPackageDescendantSize  = "package_descendant_size"
	LiveScenarioPQCommitmentOpReturn   = "pq_commitment_op_return"
	LiveScenarioPQCarrierP2SHAccept    = "pq_carrier_p2sh_accept"
)

// statefulMempoolLiveTemplates maps core_mempool_vectors.json template → live script scenario.
var statefulMempoolLiveTemplates = map[string]string{
	"dust_output_reject":        LiveScenarioDust,
	"absurd_fee":                LiveScenarioAbsurdFee,
	"non_final":                 LiveScenarioNonFinal,
	"rbf_insufficient_fee":      LiveScenarioRBFInsufficient,
	"rbf_sufficient_fee":        LiveScenarioRBFSufficient,
	"coinbase_immature":         LiveScenarioCoinbaseImmature,
	"package_ancestor_limit":    LiveScenarioPackageAncestorLimit,
	"min_relay_fee":             LiveScenarioMinRelayFee,
	"mempool_double_spend":      LiveScenarioMempoolDoubleSpend,
	"rbf_not_replaceable":       LiveScenarioRBFNotReplaceable,
	"rbf_fullrbf":               LiveScenarioRBFFullRBF,
	"package_descendant_limit":  LiveScenarioPackageDescendantLimit,
	"p2pkh_roundtrip":           LiveScenarioP2PKHRoundtrip,
	"rbf_too_many_descendants":  LiveScenarioRBFTooManyDescendants,
	"p2sh_nested_p2pkh":         LiveScenarioP2SHNestedP2PKH,
	"p2sh_multisig":             LiveScenarioP2SHMultisig,
	"bare_multisig":             LiveScenarioBareMultisig,
	"p2sh_cltv_p2pk":            LiveScenarioP2SHCLTVP2PK,
	"p2sh_csv_p2pk":             LiveScenarioP2SHCSVP2PK,
	"p2pk_non_standard_input":   LiveScenarioP2PKNonStandardInput,
	"package_ancestor_size":     LiveScenarioPackageAncestorSize,
	"package_descendant_size":   LiveScenarioPackageDescendantSize,
	"pq_commitment_op_return":   LiveScenarioPQCommitmentOpReturn,
	"pq_carrier_p2sh_accept":    LiveScenarioPQCarrierP2SHAccept,
}

// StatefulMempoolLiveScenario returns the live reboottestnet probe name for a stateful template, if any.
func StatefulMempoolLiveScenario(template string) (scenario string, live bool) {
	s, ok := statefulMempoolLiveTemplates[template]
	return s, ok
}

// StatefulMempoolLiveScenarios returns the distinct live probe names (for cert gates).
func StatefulMempoolLiveScenarios() []string {
	seen := make(map[string]struct{}, len(statefulMempoolLiveTemplates))
	var out []string
	for _, s := range statefulMempoolLiveTemplates {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
