// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"strings"

	"dogego/secp256k1"

	"dogego/wire"
)

// ScriptSigChecker performs CHECKSIG and timelock checks (Core BaseSignatureChecker).
type ScriptSigChecker interface {
	CheckSig(sig, pubKey []byte, flags ScriptVerifyFlags) (bool, ScriptError)
	SpendContext() (*wire.Tx, int)
}

type noopScriptChecker struct{}

func (noopScriptChecker) CheckSig([]byte, []byte, ScriptVerifyFlags) (bool, ScriptError) {
	return false, ScriptErrBadOpcode
}

func (noopScriptChecker) SpendContext() (*wire.Tx, int) { return nil, 0 }

func (c *ScriptSpendChecker) SpendContext() (*wire.Tx, int) {
	if c == nil || c.Tx == nil {
		return nil, 0
	}
	return c.Tx, c.InputIdx
}

// ScriptSpendChecker signs against scriptPubKey (subscript) at input 0 of Tx.
type ScriptSpendChecker struct {
	Tx           *wire.Tx
	InputIdx     int
	Subscript    []byte
	CodeSepBegin int
}

func (c *ScriptSpendChecker) setCodeSepOffset(off int) {
	if c != nil {
		c.CodeSepBegin = off
	}
}

// buildScriptTestCreditSpend mirrors Core BuildCreditingTransaction + BuildSpendingTransaction (script_tests DoTest).
func buildScriptTestCreditSpend(scriptSig, scriptPubKey []byte) (spend, credit *wire.Tx) {
	return buildScriptTestCreditSpendFlags(scriptSig, scriptPubKey, 0)
}

func buildScriptTestCreditSpendFlags(scriptSig, scriptPubKey []byte, flags ScriptVerifyFlags) (spend, credit *wire.Tx) {
	credit = &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{}, // Core COutPoint::SetNull
			PrevIdx:  0xffffffff,
			Script:   []byte{0x00, 0x00}, // CScript() << 0 << 0
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{
			Value:    0,
			PkScript: append([]byte(nil), scriptPubKey...),
		}},
	}
	h := credit.TxHash()
	spend = &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: h,
			PrevIdx:  0,
			Script:   append([]byte(nil), scriptSig...),
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{
			Value:    0,
			PkScript: nil,
		}},
	}
	// Core script_tests DoTest uses nVersion=1 and SEQUENCE_FINAL on the spend tx (no BIP68/65 overrides).
	return spend, credit
}

func (c *ScriptSpendChecker) CheckSig(sig, pubKey []byte, flags ScriptVerifyFlags) (bool, ScriptError) {
	sub := subscriptFromCodeSep(c.Subscript, c.CodeSepBegin)
	code := scriptCodeForSignature(sub, sig)
	return evalCheckSig(c.Tx, c.InputIdx, code, sig, pubKey, flags)
}

func evalCheckSig(tx *wire.Tx, idx int, subscript []byte, sig, pubKey []byte, flags ScriptVerifyFlags) (bool, ScriptError) {
	if err := checkSignatureEncoding(sig, flags); err != nil {
		return false, scriptErrFromSigEncoding(err)
	}
	if err := checkPubKeyEncoding(pubKey, flags); err != ScriptErrOK {
		return false, err
	}
	if len(sig) == 0 {
		// Core: empty signature is invalid but does not trigger NULLFAIL (BIP146).
		return false, ScriptErrOK
	}
	sigBytes, hashType, err := parseSignaturePush(sig)
	if err != nil {
		if flags&(ScriptVerifyDERSig|ScriptVerifyStrictEnc) != 0 {
			return false, ScriptErrSigDer
		}
		return false, ScriptErrOK
	}
	pub, err := secp256k1.ParsePubKey(pubKey)
	if err != nil {
		return false, ScriptErrOK
	}
	if hashType&wire.SigHashSingle == wire.SigHashSingle && idx >= len(tx.Vout) {
		return false, ScriptErrEvalFalse
	}
	digest, err := wire.CalcSignatureHashLegacy(subscript, hashType, tx, idx)
	if err != nil {
		return false, ScriptErrEvalFalse
	}
	strictDER := flags&(ScriptVerifyDERSig|ScriptVerifyLowS|ScriptVerifyStrictEnc) != 0
	var valid bool
	if strictDER {
		valid = verifyECDSASignatureStrict(sigBytes, digest[:], pub)
	} else {
		valid = verifyECDSASignatureLax(sigBytes, digest[:], pub)
		if !valid {
			valid = verifyECDSASignatureStrict(sigBytes, digest[:], pub)
		}
	}
	if !valid {
		if flags&ScriptVerifyNullFail != 0 && len(sigBytes) > 0 {
			return false, ScriptErrSigNullFail
		}
		return false, ScriptErrOK
	}
	return true, ScriptErrOK
}

func scriptErrFromSigEncoding(err error) ScriptError {
	if err == nil {
		return ScriptErrOK
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "non-DER"):
		return ScriptErrSigDer
	case strings.Contains(msg, "high-S"):
		return ScriptErrSigHighS
	case strings.Contains(msg, "undefined signature hashtype"):
		return ScriptErrSigHashType
	default:
		return ScriptErrSigDer
	}
}
