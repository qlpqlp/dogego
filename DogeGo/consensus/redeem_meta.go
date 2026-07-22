// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// RedeemScriptMeta returns optional RPC/explorer fields describing a known redeem or output script template.
func RedeemScriptMeta(script []byte) map[string]interface{} {
	if len(script) == 0 {
		return nil
	}
	if pq := PQCommitmentFields(script); pq != nil {
		return pq
	}
	if ver, ok := ParseWitnessProgram(script); ok {
		return map[string]interface{}{
			"dogego_script_template": "witness",
			"dogego_witness_version": ver,
		}
	}
	if isP2PKHScript(script) {
		return map[string]interface{}{"dogego_script_template": "pubkeyhash"}
	}
	if isP2PKScript(script) {
		return map[string]interface{}{"dogego_script_template": "pubkey"}
	}
	if IsMultisigRedeemScript(script) {
		n, _, err := ParseMultisigRedeemScript(script)
		if err != nil {
			return map[string]interface{}{"dogego_script_template": "multisig"}
		}
		return map[string]interface{}{
			"dogego_script_template": "multisig",
			"dogego_multisig_m":      n,
		}
	}
	if isCLTVP2PKHRedeem(script) {
		return map[string]interface{}{
			"dogego_script_template": "cltv_pubkeyhash",
			"dogego_timelock":        "absolute",
		}
	}
	if isCSVP2PKHRedeem(script) {
		return map[string]interface{}{
			"dogego_script_template": "csv_pubkeyhash",
			"dogego_timelock":        "relative",
		}
	}
	if isTimelockRedeem(script, opCheckLockTimeVerify) {
		return timelockRedeemMeta(script, opCheckLockTimeVerify, "absolute")
	}
	if isTimelockRedeem(script, opCheckSequenceVerify) {
		return timelockRedeemMeta(script, opCheckSequenceVerify, "relative")
	}
	if isP2SHForwardRedeem(script) {
		return map[string]interface{}{
			"dogego_script_template": "p2sh_forward",
		}
	}
	if IsPQCarrierRedeemScript(script) {
		return map[string]interface{}{
			"dogego_script_template": "pq_carrier_redeem",
			"dogego_pqc_mode":        "carrier_scriptsig",
		}
	}
	return nil
}

func timelockRedeemMeta(script []byte, timeOpcode byte, kind string) map[string]interface{} {
	_, tail, err := parseTimelockDropRedeem(script, timeOpcode)
	if err != nil {
		return nil
	}
	m := map[string]interface{}{
		"dogego_timelock": kind,
	}
	switch {
	case isP2PKHScript(tail):
		m["dogego_script_template"] = "cltv_pubkeyhash"
		if kind == "relative" {
			m["dogego_script_template"] = "csv_pubkeyhash"
		}
	case isP2PKScript(tail):
		m["dogego_script_template"] = "cltv_pubkey"
		if kind == "relative" {
			m["dogego_script_template"] = "csv_pubkey"
		}
	case IsMultisigRedeemScript(tail):
		m["dogego_script_template"] = "cltv_multisig"
		if kind == "relative" {
			m["dogego_script_template"] = "csv_multisig"
		}
		if n, _, err := ParseMultisigRedeemScript(tail); err == nil {
			m["dogego_multisig_m"] = n
		}
	}
	return m
}
