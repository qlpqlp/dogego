// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"
	"golang.org/x/crypto/ripemd160"

	"dogego/chain"
	"dogego/wire"
)

// ScriptVerifyFlags mirrors Core script verification flags used by DogeGo.
type ScriptVerifyFlags uint32

const (
	// ScriptVerifyDERSig enforces BIP66 strict DER signatures (SCRIPT_VERIFY_DERSIG).
	ScriptVerifyDERSig ScriptVerifyFlags = 1 << 2
	// ScriptVerifyCheckLockTimeVerify enforces BIP65 OP_CHECKLOCKTIMEVERIFY (SCRIPT_VERIFY_CHECKLOCKTIMEVERIFY).
	ScriptVerifyCheckLockTimeVerify ScriptVerifyFlags = 1 << 9
	// ScriptVerifyCheckSequenceVerify enforces BIP112 OP_CHECKSEQUENCEVERIFY (SCRIPT_VERIFY_CHECKSEQUENCEVERIFY).
	ScriptVerifyCheckSequenceVerify ScriptVerifyFlags = 1 << 10
)

// ScriptFlagsForHeight returns flags active at spendHeight on net (no header journal; uses buried CSVHeight).
func ScriptFlagsForHeight(spendHeight int64, net chain.Network) ScriptVerifyFlags {
	return ScriptFlagsForChain(spendHeight, net, nil)
}

// ErrMissingPrevout is returned when a spend cannot be resolved for script verification.
var ErrMissingPrevout = errors.New("consensus: missing prevout for script verification")

// ErrWitnessNotSupported is returned for segwit transactions.
var ErrWitnessNotSupported = errors.New("consensus: witness transactions are not supported")

// VerifyScript checks each input against its prevout scriptPubKey with no height-dependent flags.
func VerifyScript(tx *wire.Tx, view PrevOutView) error {
	return VerifyScriptFlags(tx, view, 0)
}

// VerifyScriptFlags checks inputs with Core-style script flags (e.g. BIP66 DER at height).
func VerifyScriptFlags(tx *wire.Tx, view PrevOutView, flags ScriptVerifyFlags) error {
	if err := CheckTransaction(tx, true); err != nil {
		return err
	}
	if IsCoinbaseTx(tx) {
		return nil
	}
	if tx.HasWitness() {
		return ErrWitnessNotSupported
	}
	for i := range tx.Vin {
		prev, ok := view.Lookup(tx.Vin[i].PrevHash, tx.Vin[i].PrevIdx)
		if !ok {
			return fmt.Errorf("%w (input %d)", ErrMissingPrevout, i)
		}
		if err := checkInputDiscouragedOps(tx, i, prev.PkScript, flags); err != nil {
			return fmt.Errorf("input %d: %w", i, err)
		}
		if err := verifyInput(tx, i, prev.PkScript, flags); err != nil {
			return fmt.Errorf("input %d: %w", i, err)
		}
	}
	return nil
}

// VerifyScriptAtHeight applies ScriptFlagsForChain then VerifyScriptFlags.
func VerifyScriptAtHeight(tx *wire.Tx, view PrevOutView, spendHeight int64, net chain.Network, journal HeaderChain) error {
	return VerifyScriptFlags(tx, view, ScriptFlagsForChain(spendHeight, net, journal))
}

func verifyInput(tx *wire.Tx, idx int, pkScript []byte, flags ScriptVerifyFlags) error {
	switch {
	case isP2PKHScript(pkScript):
		return verifyInputP2PKH(tx, idx, pkScript, flags)
	case isP2PKScript(pkScript):
		return verifyInputP2PK(tx, idx, pkScript, flags)
	case isP2SHScript(pkScript):
		return verifyInputP2SH(tx, idx, pkScript, flags)
	case IsMultisigRedeemScript(pkScript):
		return verifyInputBareMultisig(tx, idx, pkScript, flags)
	default:
		return verifyInputEval(tx, idx, pkScript, flags)
	}
}

func verifyInputP2PKH(tx *wire.Tx, idx int, pkScript []byte, flags ScriptVerifyFlags) error {
	return verifyInputP2PKHScriptSig(tx, idx, pkScript, tx.Vin[idx].Script, flags)
}

func verifyInputP2PKHScriptSig(tx *wire.Tx, idx int, pkScript, scriptSig []byte, flags ScriptVerifyFlags) error {
	if !isP2PKHScript(pkScript) {
		return fmt.Errorf("script-verify: expected P2PKH script")
	}
	if !isPushOnly(scriptSig) {
		return fmt.Errorf("script-verify: scriptSig is not push-only")
	}
	if flags&ScriptVerifyMinimalData != 0 {
		if err := checkScriptMinimalData(scriptSig); err != nil {
			return err
		}
	}
	sig, pubKey, err := parseP2PKHScriptSig(scriptSig)
	if err != nil {
		return err
	}
	var wantHash160 [20]byte
	copy(wantHash160[:], pkScript[3:23])
	if hash160(pubKey) != wantHash160 {
		return fmt.Errorf("script-verify: pubkey hash mismatch")
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

func isP2PKHScript(s []byte) bool {
	return len(s) == 25 && s[0] == 0x76 && s[1] == 0xa9 && s[2] == 0x14 &&
		s[23] == 0x88 && s[24] == 0xac
}

func isP2SHScript(s []byte) bool {
	return len(s) == 23 && s[0] == 0xa9 && s[1] == 0x14 && s[22] == 0x87
}

func verifyInputBareMultisig(tx *wire.Tx, idx int, pkScript []byte, flags ScriptVerifyFlags) error {
	if !IsMultisigRedeemScript(pkScript) {
		return fmt.Errorf("script-verify: expected bare multisig scriptPubKey")
	}
	if !isPushOnly(tx.Vin[idx].Script) {
		return fmt.Errorf("script-verify: scriptSig is not push-only")
	}
	if flags&ScriptVerifyMinimalData != 0 {
		if err := checkScriptMinimalData(tx.Vin[idx].Script); err != nil {
			return err
		}
	}
	pushes, err := allScriptPushes(tx.Vin[idx].Script)
	if err != nil {
		return err
	}
	return verifyMultisigRedeem(tx, idx, pkScript, pushes, flags)
}

func verifyInputP2SH(tx *wire.Tx, idx int, pkScript []byte, flags ScriptVerifyFlags) error {
	if !isP2SHScript(pkScript) {
		return fmt.Errorf("script-verify: expected P2SH script")
	}
	if !isPushOnly(tx.Vin[idx].Script) {
		return fmt.Errorf("script-verify: scriptSig is not push-only")
	}
	if flags&ScriptVerifyMinimalData != 0 {
		if err := checkScriptMinimalData(tx.Vin[idx].Script); err != nil {
			return err
		}
	}
	pushes, err := allScriptPushes(tx.Vin[idx].Script)
	if err != nil {
		return err
	}
	if len(pushes) < 2 {
		return fmt.Errorf("script-verify: p2sh scriptSig too short")
	}
	redeem := pushes[len(pushes)-1]
	var wantH [20]byte
	copy(wantH[:], pkScript[2:22])
	if hash160(redeem) != wantH {
		return fmt.Errorf("script-verify: p2sh hash mismatch")
	}
	if IsPQCarrierRedeemScript(redeem) {
		return verifyInputEval(tx, idx, pkScript, flags)
	}
	sigPushes := pushes[:len(pushes)-1]
	return verifyP2SHRedeemScript(tx, idx, redeem, sigPushes, flags)
}

func allScriptPushes(script []byte) ([][]byte, error) {
	var out [][]byte
	i := 0
	for i < len(script) {
		data, n, err := ReadScriptPush(script, i)
		if err != nil {
			return nil, err
		}
		out = append(out, data)
		i = n
	}
	return out, nil
}

func buildP2PKHScriptSig(sig, pubKey []byte) []byte {
	var b []byte
	b = append(b, byte(len(sig)))
	b = append(b, sig...)
	b = append(b, byte(len(pubKey)))
	b = append(b, pubKey...)
	return b
}

func buildSinglePushScript(data []byte) []byte {
	if len(data) <= 75 {
		b := make([]byte, 0, 1+len(data))
		b = append(b, byte(len(data)))
		return append(b, data...)
	}
	b := make([]byte, 0, 2+len(data))
	b = append(b, 0x4c, byte(len(data)))
	return append(b, data...)
}

func lastScriptPush(script []byte) ([]byte, error) {
	return LastScriptPush(script)
}

func isPushOnly(script []byte) bool {
	i := 0
	for i < len(script) {
		op := script[i]
		i++
		switch {
		case op == 0x00:
			continue
		case op >= 0x01 && op <= 0x4b:
			if i+int(op) > len(script) {
				return false
			}
			i += int(op)
		case op == 0x4c:
			if i >= len(script) {
				return false
			}
			n := int(script[i])
			i++
			if i+n > len(script) {
				return false
			}
			i += n
		case op == 0x4d:
			if i+1 >= len(script) {
				return false
			}
			n := int(script[i]) | int(script[i+1])<<8
			i += 2
			if i+n > len(script) {
				return false
			}
			i += n
		case op == 0x4e:
			if i+3 >= len(script) {
				return false
			}
			n := int(script[i]) | int(script[i+1])<<8 | int(script[i+2])<<16 | int(script[i+3])<<24
			i += 4
			if i+n > len(script) {
				return false
			}
			i += n
		case op == 0x4f, op >= 0x51 && op <= 0x60:
			continue
		default:
			return false
		}
	}
	return i == len(script)
}

func parseP2PKHScriptSig(script []byte) (sig, pubKey []byte, err error) {
	var parts [][]byte
	i := 0
	for i < len(script) && len(parts) < 2 {
		data, n, err := ReadScriptPush(script, i)
		if err != nil {
			return nil, nil, err
		}
		parts = append(parts, data)
		i = n
	}
	if len(parts) != 2 || i != len(script) {
		return nil, nil, fmt.Errorf("script-verify: expected signature and pubkey pushes")
	}
	return parts[0], parts[1], nil
}

func parseSignaturePush(sig []byte) (der []byte, hashType uint32, err error) {
	if len(sig) < 2 {
		return nil, 0, fmt.Errorf("script-verify: signature too short")
	}
	hashType = uint32(sig[len(sig)-1])
	der = sig[:len(sig)-1]
	return der, hashType, nil
}

func hash160(b []byte) [20]byte {
	sh := sha256.Sum256(b)
	r := ripemd160.New()
	_, _ = r.Write(sh[:])
	var out [20]byte
	copy(out[:], r.Sum(nil))
	return out
}
