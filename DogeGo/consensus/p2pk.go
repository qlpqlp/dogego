// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/wire"
)

// isP2PKScript matches <pubkey> OP_CHECKSIG (compressed or uncompressed).
func isP2PKScript(s []byte) bool {
	pub, err := parseP2PKPubKeyScript(s)
	return err == nil && len(pub) > 0
}

func parseP2PKPubKeyScript(pkScript []byte) ([]byte, error) {
	if len(pkScript) < 3 || pkScript[len(pkScript)-1] != 0xac {
		return nil, fmt.Errorf("p2pk: missing OP_CHECKSIG")
	}
	pub, next, err := ReadScriptPush(pkScript, 0)
	if err != nil {
		return nil, err
	}
	if next != len(pkScript)-1 {
		return nil, fmt.Errorf("p2pk: trailing script ops")
	}
	if len(pub) != 33 && len(pub) != 65 {
		return nil, fmt.Errorf("p2pk: bad pubkey length")
	}
	if _, err := secp256k1.ParsePubKey(pub); err != nil {
		return nil, fmt.Errorf("p2pk: invalid pubkey: %w", err)
	}
	return pub, nil
}

func verifyInputP2PK(tx *wire.Tx, idx int, pkScript []byte, flags ScriptVerifyFlags) error {
	return verifyInputP2PKScriptSig(tx, idx, pkScript, tx.Vin[idx].Script, flags)
}

func verifyInputP2PKScriptSig(tx *wire.Tx, idx int, pkScript, scriptSig []byte, flags ScriptVerifyFlags) error {
	pubKey, err := parseP2PKPubKeyScript(pkScript)
	if err != nil {
		return err
	}
	if !isPushOnly(scriptSig) {
		return fmt.Errorf("script-verify: scriptSig is not push-only")
	}
	if flags&ScriptVerifyMinimalData != 0 {
		if err := checkScriptMinimalData(scriptSig); err != nil {
			return err
		}
	}
	sig, err := parseSinglePush(scriptSig)
	if err != nil {
		return err
	}
	pub, err := secp256k1.ParsePubKey(pubKey)
	if err != nil {
		return fmt.Errorf("script-verify: invalid pubkey")
	}
	sigBytes, hashType, err := parseSignaturePush(sig)
	if err != nil {
		return err
	}
	if err := checkSignatureEncoding(sig, flags); err != nil {
		return err
	}
	if hashType&wire.SigHashSingle == wire.SigHashSingle && idx >= len(tx.Vout) {
		return fmt.Errorf("script-verify: invalid sighash single")
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, hashType, tx, idx)
	if err != nil {
		return err
	}
	s, err := ecdsa.ParseDERSignature(sigBytes)
	if err != nil {
		return fmt.Errorf("script-verify: invalid signature encoding")
	}
	if !s.Verify(digest[:], pub) {
		if flags&ScriptVerifyNullFail != 0 && len(sigBytes) > 0 {
			return fmt.Errorf("script-verify: NULLFAIL")
		}
		return fmt.Errorf("script-verify: signature failed")
	}
	return nil
}

func parseSinglePush(script []byte) ([]byte, error) {
	pushes, err := allScriptPushes(script)
	if err != nil {
		return nil, err
	}
	if len(pushes) != 1 {
		return nil, fmt.Errorf("script-verify: p2pk scriptSig must be one push")
	}
	return pushes[0], nil
}
