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

// IsMultisigRedeemScript reports OP_M … OP_N OP_CHECKMULTISIG (standard bare multisig template).
func IsMultisigRedeemScript(script []byte) bool {
	_, _, err := ParseMultisigRedeemScript(script)
	return err == nil
}

// ParseMultisigRedeemScript decodes a standard multisig redeem script (createmultisig layout).
func ParseMultisigRedeemScript(script []byte) (nRequired int, pubkeys [][]byte, err error) {
	if len(script) < 3 {
		return 0, nil, fmt.Errorf("multisig: script too short")
	}
	if script[len(script)-1] != 0xae {
		return 0, nil, fmt.Errorf("multisig: missing OP_CHECKMULTISIG")
	}
	opM := script[0]
	if opM < 0x51 || opM > 0x60 {
		return 0, nil, fmt.Errorf("multisig: bad OP_M")
	}
	nRequired = int(opM - 0x50)
	i := 1
	var keys [][]byte
	for k := 0; k < 16 && i < len(script)-2; k++ {
		pk, next, err := ReadScriptPush(script, i)
		if err != nil {
			return 0, nil, err
		}
		if len(pk) != 33 && len(pk) != 65 {
			break
		}
		if _, err := secp256k1.ParsePubKey(pk); err != nil {
			return 0, nil, fmt.Errorf("multisig: invalid pubkey")
		}
		keys = append(keys, pk)
		i = next
	}
	if len(keys) < nRequired {
		return 0, nil, fmt.Errorf("multisig: not enough pubkeys")
	}
	if i >= len(script) {
		return 0, nil, fmt.Errorf("multisig: missing OP_N")
	}
	opN := script[i]
	if opN < 0x51 || opN > 0x60 {
		return 0, nil, fmt.Errorf("multisig: bad OP_N")
	}
	if int(opN-0x50) != len(keys) {
		return 0, nil, fmt.Errorf("multisig: OP_N mismatch")
	}
	i++
	if i != len(script)-1 || script[i] != 0xae {
		return 0, nil, fmt.Errorf("multisig: trailing bytes")
	}
	return nRequired, keys, nil
}

// verifyMultisigRedeem checks signatures in scriptSig pushes against a bare multisig redeem script.
func verifyMultisigRedeem(tx *wire.Tx, inputIdx int, redeem []byte, sigPushes [][]byte, flags ScriptVerifyFlags) error {
	nReq, pubkeys, err := ParseMultisigRedeemScript(redeem)
	if err != nil {
		return err
	}
	sigs := sigPushes
	if len(sigs) == 0 {
		return fmt.Errorf("script-verify: multisig scriptSig missing pushes")
	}
	if flags&ScriptVerifyNullDummy != 0 && len(sigs[0]) != 0 {
		return fmt.Errorf("script-verify: SIG_NULLDUMMY")
	}
	if len(sigs[0]) == 0 {
		sigs = sigs[1:]
	}
	if len(sigs) < nReq {
		return fmt.Errorf("script-verify: not enough multisig signatures")
	}
	isig := 0
	ikey := 0
	nSigs := len(sigs)
	nKeys := len(pubkeys)
	for nSigs > 0 && nKeys > 0 {
		if err := checkMultisigSignature(tx, inputIdx, redeem, sigs[isig], pubkeys[ikey], flags); err == nil {
			isig++
			nSigs--
		}
		ikey++
		nKeys--
		if nSigs > nKeys {
			return fmt.Errorf("script-verify: multisig signature check failed")
		}
	}
	if nSigs != 0 {
		return fmt.Errorf("script-verify: unused multisig signatures remain")
	}
	return nil
}

func checkMultisigSignature(tx *wire.Tx, inputIdx int, redeem, sigPush, pubKey []byte, flags ScriptVerifyFlags) error {
	der, hashType, err := parseSignaturePush(sigPush)
	if err != nil {
		return err
	}
	if err := checkSignatureEncoding(sigPush, flags); err != nil {
		return err
	}
	pub, err := secp256k1.ParsePubKey(pubKey)
	if err != nil {
		return fmt.Errorf("script-verify: invalid pubkey")
	}
	if hashType&wire.SigHashSingle == wire.SigHashSingle && inputIdx >= len(tx.Vout) {
		return fmt.Errorf("script-verify: invalid sighash single")
	}
	digest, err := wire.CalcSignatureHashLegacy(redeem, hashType, tx, inputIdx)
	if err != nil {
		return err
	}
	s, err := ecdsa.ParseDERSignature(der)
	if err != nil {
		return fmt.Errorf("script-verify: invalid signature encoding")
	}
	if !s.Verify(digest[:], pub) {
		der, _, _ := parseSignaturePush(sigPush)
		if flags&ScriptVerifyNullFail != 0 && len(der) > 0 {
			return fmt.Errorf("script-verify: NULLFAIL")
		}
		return fmt.Errorf("script-verify: signature failed")
	}
	return nil
}
