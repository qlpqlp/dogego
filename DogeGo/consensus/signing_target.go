// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "fmt"

// SigningScriptAndRedeem resolves legacy sighash scriptCode for signrawtransaction.
// For nested P2SH-forward redeems, innerRedeemScript must be the final redeem (P2PKH/P2PK/multisig/...).
// redeemPushes lists script pushes to append to scriptSig after signatures (inner first, outer last).
func SigningScriptAndRedeem(scriptPubKey, redeemScript, innerRedeemScript []byte) (scriptCode []byte, p2sh bool, redeemPushes [][]byte, err error) {
	switch {
	case isP2PKHScript(scriptPubKey):
		return scriptPubKey, false, nil, nil
	case isP2PKScript(scriptPubKey):
		return scriptPubKey, false, nil, nil
	case IsMultisigRedeemScript(scriptPubKey):
		return scriptPubKey, false, nil, nil
	case isP2SHScript(scriptPubKey):
		if len(redeemScript) == 0 {
			return nil, false, nil, fmt.Errorf("redeemScript required for P2SH prevout")
		}
		code, pushes, err := signingScriptCodeFromRedeem(redeemScript, innerRedeemScript)
		if err != nil {
			return nil, false, nil, err
		}
		return code, true, pushes, nil
	default:
		return nil, false, nil, fmt.Errorf("unsupported scriptPubKey for signing")
	}
}

func signingScriptCodeFromRedeem(redeem, inner []byte) ([]byte, [][]byte, error) {
	if isP2SHForwardRedeem(redeem) {
		if len(inner) == 0 {
			return nil, nil, fmt.Errorf("innerRedeemScript required for nested P2SH forward redeem")
		}
		code, tail, err := signingScriptCodeFromRedeem(inner, nil)
		if err != nil {
			return nil, nil, err
		}
		tail = append(tail, redeem)
		return code, tail, nil
	}
	code, err := signingScriptCodeFromRedeemSimple(redeem)
	if err != nil {
		return nil, nil, err
	}
	return code, [][]byte{redeem}, nil
}

func signingScriptCodeFromRedeemSimple(redeem []byte) ([]byte, error) {
	switch {
	case isP2PKHScript(redeem), isP2PKScript(redeem):
		return redeem, nil
	case isCLTVP2PKHRedeem(redeem):
		_, inner, err := parseCLTVP2PKHRedeem(redeem)
		return inner, err
	case isCSVP2PKHRedeem(redeem):
		_, inner, err := parseCSVP2PKHRedeem(redeem)
		return inner, err
	case IsMultisigRedeemScript(redeem):
		return redeem, nil
	case IsDoginalLockRedeem(redeem):
		// apezord/doginals: sighash scriptCode is the full lock redeem.
		return redeem, nil
	default:
		return signingScriptFromTimelockInner(redeem)
	}
}

func signingScriptFromTimelockInner(redeem []byte) ([]byte, error) {
	if isTimelockRedeem(redeem, opCheckLockTimeVerify) {
		_, tail, err := parseTimelockDropRedeem(redeem, opCheckLockTimeVerify)
		if err == nil && (isP2PKHScript(tail) || isP2PKScript(tail) || IsMultisigRedeemScript(tail)) {
			return tail, nil
		}
	}
	if isTimelockRedeem(redeem, opCheckSequenceVerify) {
		_, tail, err := parseTimelockDropRedeem(redeem, opCheckSequenceVerify)
		if err == nil && (isP2PKHScript(tail) || isP2PKScript(tail) || IsMultisigRedeemScript(tail)) {
			return tail, nil
		}
	}
	return nil, fmt.Errorf("unsupported redeemScript for signing")
}
