// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"fmt"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

type coreScriptVector struct {
	Name       string `json:"name"`
	Template   string `json:"template"`
	WantAccept bool   `json:"want_accept"`
}

func loadCoreScriptVectors(t *testing.T) []coreScriptVector {
	t.Helper()
	var vecs []coreScriptVector
	loadJSONFixture(t, "core_script_vectors.json", &vecs)
	if len(vecs) == 0 {
		t.Fatal("no script differential vectors loaded")
	}
	return vecs
}

// TestCoreScriptDifferentialVectors exercises supported legacy script paths (P2PKH, P2PK, nested P2SH).
// Broader coverage lives in TestCoreScriptTestsRunnerSubset (script_tests.json; witness rows skipped).
func TestCoreScriptDifferentialVectors(t *testing.T) {
	for _, v := range loadCoreScriptVectors(t) {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			runScriptPathVector(t, v.Template, v.WantAccept)
		})
	}
}

// TestCoreScriptDifferentialSuiteIncludesLegacyPaths runs additional script tests from the tree.
func TestCoreScriptDifferentialSuiteIncludesLegacyPaths(t *testing.T) {
	t.Run("p2pk", func(t *testing.T) { TestVerifyScriptP2PKRoundTrip(t) })
	t.Run("p2sh_nested", func(t *testing.T) { TestVerifyScriptNestedP2SHP2PKH(t) })
	t.Run("p2sh_multisig", func(t *testing.T) { TestVerifyScriptP2SHMultisig(t) })
	t.Run("bare_multisig", func(t *testing.T) { TestVerifyScriptBareMultisig(t) })
	t.Run("p2sh_cltv", func(t *testing.T) { TestVerifyScriptP2SHCLTVP2PK(t) })
}

func runScriptPathVector(t *testing.T, tmpl string, wantAccept bool) {
	t.Helper()
	err := evalScriptDifferentialTemplate(tmpl)
	if wantAccept {
		if err != nil {
			t.Fatalf("expected accept: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatal("expected reject")
	}
}

func evalScriptDifferentialTemplate(tmpl string) error {
	var err error
	switch tmpl {
	case "p2pkh_roundtrip":
		err = verifyP2PKHRoundTripScript(0x33)
	case "p2pk_roundtrip":
		err = verifyP2PKRoundTripScript(0x42)
	case "p2sh_nested_p2pkh":
		err = verifyP2SHNestedP2PKHScript(0x44)
	case "p2sh_multisig":
		err = verifyP2SHMultisigScript(0x77)
	case "bare_multisig":
		err = verifyBareMultisigScript(0x88)
	case "p2sh_cltv_p2pk":
		err = verifyP2SHCLTVP2PKScript(0x47)
	case "p2sh_csv_p2pk":
		err = verifyP2SHCSVP2PKScript(0x55)
	case "op_if_else_true":
		err = verifyOPIfElseScript(true)
	case "op_if_else_false":
		err = verifyOPIfElseScript(false)
	case "op_notif_true_branch":
		err = verifyOPNotIfScript(false)
	case "op_notif_false_branch":
		err = verifyOPNotIfScript(true)
	case "op_verify_true":
		err = verifyOPVerifyScript(true)
	case "op_verify_false":
		err = verifyOPVerifyScript(false)
	case "op_toaltstack_roundtrip":
		err = verifyOPToAltStackRoundtrip()
	case "op_fromaltstack_empty":
		err = verifyOPFromAltStackEmpty()
	case "op_pick":
		err = verifyOPPickScript()
	case "op_depth":
		err = verifyOPDepthScript()
	case "op_drop_empty":
		err = verifyOPDropEmptyScript()
	case "op_roll":
		err = verifyOPRollScript()
	case "op_rot":
		err = verifyOPRotScript()
	case "op_nested_if":
		err = verifyOPNestedIfScript(true)
	case "op_nested_if_false":
		err = verifyOPNestedIfScript(false)
	case "op_disabled_cat":
		err = verifyOPDisabledCatScript()
	case "op_over":
		err = verifyOPOverScript()
	case "op_2dup":
		err = verifyOP2DupScript()
	case "op_ifdup":
		err = verifyOPIfDupScript()
	case "op_unbalanced_if":
		err = verifyOPUnbalancedIfScript()
	case "op_swap":
		err = verifyOPSwapScript()
	case "op_tuck":
		err = verifyOPTuckScript()
	case "op_nip":
		err = verifyOPNipScript()
	case "op_3dup":
		err = verifyOP3DupScript()
	case "op_equal":
		err = verifyOPEqualScript(true)
	case "op_equal_false":
		err = verifyOPEqualScript(false)
	case "op_size":
		err = verifyOPSizeScript()
	case "op_2over":
		err = verifyOP2OverScript()
	case "op_booland":
		err = verifyOPBoolAndScript(true)
	case "op_boolor":
		err = verifyOPBoolOrScript(true)
	case "op_numequal":
		err = verifyOPNumEqualScript(true)
	case "op_equalverify":
		err = verifyOPEqualVerifyScript(true)
	case "op_2swap":
		err = verifyOP2SwapScript()
	case "op_not":
		err = verifyOPNotScript()
	case "op_1add":
		err = verifyOP1AddScript()
	case "op_2rot":
		err = verifyOP2RotScript()
	case "op_numequal_false":
		err = verifyOPNumEqualScript(false)
	case "op_numnotequal":
		err = verifyOPNumNotEqualScript(true)
	case "op_add":
		err = verifyOPAddScript()
	case "op_negate":
		err = verifyOPNegateScript()
	case "op_booland_false":
		err = verifyOPBoolAndScript(false)
	case "op_return":
		err = verifyOPReturnScript()
	case "op_sub":
		err = verifyOPSubScript()
	case "op_1sub":
		err = verifyOP1SubScript()
	case "op_lessthan":
		err = verifyOPLessThanScript()
	case "op_greaterthan_false":
		err = verifyOPGreaterThanScript(false)
	case "op_lessthanorequal":
		err = verifyOPLessThanOrEqualScript(true)
	case "op_greaterthanorequal":
		err = verifyOPGreaterThanOrEqualScript(true)
	case "op_greaterthanorequal_false":
		err = verifyOPGreaterThanOrEqualScript(false)
	case "op_within":
		err = verifyOPWithinScript(true)
	case "op_disabled_mul":
		err = verifyOPDisabledMulScript()
	case "op_min":
		err = verifyOPMinScript()
	case "op_max":
		err = verifyOPMaxScript()
	case "op_disabled_div":
		err = verifyOPDisabledDivScript()
	case "op_within_false":
		err = verifyOPWithinScript(false)
	case "op_lessthanorequal_false":
		err = verifyOPLessThanOrEqualScript(false)
	case "op_greaterthan":
		err = verifyOPGreaterThanScript(true)
	case "op_numnotequal_false":
		err = verifyOPNumNotEqualScript(false)
	case "op_disabled_mod":
		err = verifyOPDisabledModScript()
	case "op_codeseparator":
		err = verifyOPCodeSeparatorScript()
	case "op_numequalverify":
		err = verifyOPNumEqualVerifyScript(true)
	case "op_equalverify_false":
		err = verifyOPEqualVerifyScript(false)
	case "op_disabled_lshift":
		err = verifyOPDisabledLShiftScript()
	case "op_disabled_rshift":
		err = verifyOPDisabledRShiftScript()
	case "op_numequalverify_false":
		err = verifyOPNumEqualVerifyScript(false)
	case "op_disabled_2mul":
		err = verifyOPDisabled2MulScript()
	case "op_disabled_and":
		err = verifyOPDisabledAndScript()
	case "op_boolor_false":
		err = verifyOPBoolOrScript(false)
	case "op_disabled_or":
		err = verifyOPDisabledOrScript()
	case "op_disabled_xor":
		err = verifyOPDisabledXorScript()
	case "op_disabled_2div":
		err = verifyOPDisabled2DivScript()
	case "op_drop":
		err = verifyOPDropScript()
	case "op_dup":
		err = verifyOPDupScript()
	case "op_disabled_left":
		err = verifyOPDisabledLeftScript()
	case "op_disabled_right":
		err = verifyOPDisabledRightScript()
	case "op_disabled_invert":
		err = verifyOPDisabledInvertScript()
	case "op_disabled_substr":
		err = verifyOPDisabledSubstrScript()
	case "op_2drop":
		err = verifyOP2DropScript()
	case "op_abs":
		err = verifyOPAbsScript()
	case "op_0notequal":
		err = verifyOP0NotEqualScript(true)
	case "op_0notequal_false":
		err = verifyOP0NotEqualScript(false)
	case "op_reserved":
		err = verifyOPReservedScript()
	case "op_nop":
		err = verifyOPNopScript()
	case "op_1negate":
		err = verifyOP1NegateScript()
	case "op_ver":
		err = verifyOPVerScript()
	case "op_reserved1":
		err = verifyOPReserved1Script()
	case "op_reserved2":
		err = verifyOPReserved2Script()
	case "op_else_unbalanced":
		err = verifyOPElseUnbalancedScript()
	case "op_endif_unbalanced":
		err = verifyOPEndifUnbalancedScript()
	case "op_16":
		err = verifyOP16Script()
	case "op_2":
		err = verifyOP2Script()
	case "op_15":
		err = verifyOP15Script()
	case "op_if_empty_stack":
		err = verifyOPIfEmptyStackScript()
	case "op_verify_empty_stack":
		err = verifyOPVerifyEmptyStackScript()
	case "op_pick_underflow":
		err = verifyOPPickUnderflowScript()
	case "op_3":
		err = verifyOP3Script()
	case "op_10":
		err = verifyOP10Script()
	case "op_notif_empty_stack":
		err = verifyOPNotifEmptyStackScript()
	case "op_roll_underflow":
		err = verifyOPRollUnderflowScript()
	case "op_depth_empty":
		err = verifyOPDepthEmptyScript()
	case "op_4":
		err = verifyOP4Script()
	case "op_5":
		err = verifyOP5Script()
	case "op_6":
		err = verifyOP6Script()
	case "op_7":
		err = verifyOP7Script()
	case "op_8":
		err = verifyOP8Script()
	case "op_equalverify_empty":
		err = verifyOPEqualVerifyEmptyScript()
	case "op_numequalverify_empty":
		err = verifyOPNumEqualVerifyEmptyScript()
	case "op_9":
		err = verifyOPSmallIntEqualScript(9)
	case "op_11":
		err = verifyOPSmallIntEqualScript(11)
	case "op_12":
		err = verifyOPSmallIntEqualScript(12)
	case "op_13":
		err = verifyOPSmallIntEqualScript(13)
	case "op_14":
		err = verifyOPSmallIntEqualScript(14)
	case "op_over_underflow":
		err = verifyOPOverUnderflowScript()
	case "op_tuck_underflow":
		err = verifyOPTuckUnderflowScript()
	case "op_rot_underflow":
		err = verifyOPRotUnderflowScript()
	case "op_2drop_underflow":
		err = verifyOP2DropUnderflowScript()
	case "op_2swap_underflow":
		err = verifyOP2SwapUnderflowScript()
	case "op_dup_empty":
		err = verifyOPDupEmptyScript()
	case "op_swap_underflow":
		err = verifyOPSwapUnderflowScript()
	case "op_nip_underflow":
		err = verifyOPNipUnderflowScript()
	case "op_ifdup_empty":
		err = verifyOPIfDupEmptyScript()
	case "op_2over_underflow":
		err = verifyOP2OverUnderflowScript()
	case "op_3dup_underflow":
		err = verifyOP3DupUnderflowScript()
	case "op_2rot_underflow":
		err = verifyOP2RotUnderflowScript()
	case "op_2dup_underflow":
		err = verifyOP2DupUnderflowScript()
	case "op_toaltstack_empty":
		err = verifyOPToAltStackEmptyScript()
	case "op_2drop_empty":
		err = verifyOP2DropEmptyScript()
	default:
		return fmt.Errorf("unknown script template %q", tmpl)
	}
	return err
}

func verifyP2PKHRoundTripScript(secByte byte) error {
	sec := make([]byte, 32)
	sec[0] = secByte
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := standardP2PKHScript(h160[:])

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	pool := mempool.New(100)
	raw, err := funding.Serialize()
	if err != nil {
		return err
	}
	if err := pool.Add(raw); err != nil {
		return err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	return VerifyScript(spend, &MempoolPrevOutView{Pool: pool})
}

func verifyP2PKRoundTripScript(secByte byte) error {
	sec := make([]byte, 32)
	sec[0] = secByte
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	pkScript := append([]byte{0x21}, pubC...)
	pkScript = append(pkScript, 0xac)
	h160 := hash160(pubC)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{7}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	pool := mempool.New(10)
	raw, err := funding.Serialize()
	if err != nil {
		return err
	}
	if err := pool.Add(raw); err != nil {
		return err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildSinglePushScript(append(sig.Serialize(), byte(wire.SigHashAll)))
	return VerifyScript(spend, &MempoolPrevOutView{Pool: pool})
}

func verifyP2SHNestedP2PKHScript(secByte byte) error {
	sec := make([]byte, 32)
	sec[0] = secByte
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	innerRedeem := standardP2PKHScript(h160[:])
	forward := p2shScriptPubKey(innerRedeem)
	outerP2SH := p2shScriptPubKey(forward)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{11}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: outerP2SH}},
	}
	pool := mempool.New(50)
	raw, err := funding.Serialize()
	if err != nil {
		return err
	}
	if err := pool.Add(raw); err != nil {
		return err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	digest, err := wire.CalcSignatureHashLegacy(innerRedeem, wire.SigHashAll, spend, 0)
	if err != nil {
		return err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var scriptErr error
	spend.Vin[0].Script, scriptErr = concatScriptPushes(sigBytes, pubC, innerRedeem, forward)
	if scriptErr != nil {
		return scriptErr
	}
	flags := ScriptFlagsForHeight(4_000_000, chain.MainnetDogecoin)
	return VerifyScriptFlags(spend, &MempoolPrevOutView{Pool: pool}, flags)
}

func verifyP2SHMultisigScript(secByte byte) error {
	sec := make([]byte, 32)
	sec[0] = secByte
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildTestMultisigRedeem(1, pubC)
	rh := hash160(redeem)
	p2sh := append([]byte{0xa9, 0x14}, rh[:]...)
	p2sh = append(p2sh, 0x87)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xab}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(10)
	raw, _ := funding.Serialize()
	if err := pool.Add(raw); err != nil {
		return err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: []byte{0x51}}},
	}
	digest, _ := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()
	return VerifyScript(spend, &MempoolPrevOutView{Pool: pool})
}

func verifyBareMultisigScript(secByte byte) error {
	sec := make([]byte, 32)
	sec[0] = secByte
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildTestMultisigRedeem(1, pubC)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xcd}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: redeem}},
	}
	pool := mempool.New(10)
	raw, _ := funding.Serialize()
	if err := pool.Add(raw); err != nil {
		return err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: []byte{0x51}}},
	}
	digest, _ := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	spend.Vin[0].Script = script.Bytes()
	return VerifyScript(spend, &MempoolPrevOutView{Pool: pool})
}

func verifyP2SHCLTVP2PKScript(secByte byte) error {
	sec := make([]byte, 32)
	sec[0] = secByte
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	lockHeight := int64(700)
	redeem := buildCLTVP2PKRedeemScript(lockHeight, pubC)
	p2sh := p2shScriptPubKey(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{12}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(50)
	raw, _ := funding.Serialize()
	if err := pool.Add(raw); err != nil {
		return err
	}
	spend := &wire.Tx{
		Version:  1,
		LockTime: uint32(lockHeight),
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xfffffffe,
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: []byte{0x51}}},
	}
	_, inner, err := parseTimelockDropRedeem(redeem, opCheckLockTimeVerify)
	if err != nil {
		return err
	}
	digest, err := wire.CalcSignatureHashLegacy(inner, wire.SigHashAll, spend, 0)
	if err != nil {
		return err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()
	flags := ScriptFlagsForHeight(4_000_000, chain.MainnetDogecoin)
	return VerifyScriptFlags(spend, &MempoolPrevOutView{Pool: pool}, flags)
}

func verifyP2SHCSVP2PKScript(secByte byte) error {
	sec := make([]byte, 32)
	sec[0] = secByte
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	relSeq := int64(2)
	redeem := buildCSVP2PKRedeemScript(relSeq, pubC)
	p2sh := p2shScriptPubKey(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{13}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(50)
	raw, _ := funding.Serialize()
	if err := pool.Add(raw); err != nil {
		return err
	}
	spend := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: uint32(relSeq),
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: []byte{0x51}}},
	}
	_, inner, err := parseTimelockDropRedeem(redeem, opCheckSequenceVerify)
	if err != nil {
		return err
	}
	digest, err := wire.CalcSignatureHashLegacy(inner, wire.SigHashAll, spend, 0)
	if err != nil {
		return err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()
	flags := ScriptFlagsForHeight(500_000, chain.MainnetDogecoin)
	return VerifyScriptFlags(spend, &MempoolPrevOutView{Pool: pool}, flags)
}

// opIfElseScriptPubKey is IF OP_1 ELSE OP_0 ENDIF (Core conditional branch smoke test).
func opIfElseScriptPubKey() []byte {
	return []byte{0x63, 0x51, 0x67, 0x00, 0x68}
}

func verifyOPIfElseScript(takeIfBranch bool) error {
	var scriptSig []byte
	if takeIfBranch {
		scriptSig = []byte{0x51} // OP_1
	} else {
		scriptSig = []byte{0x00} // OP_0
	}
	return scriptErrorToGo(VerifyScriptTest(scriptSig, opIfElseScriptPubKey(), ScriptVerifyMinimalData))
}

// opNotIfScriptPubKey is NOTIF OP_1 ELSE OP_0 ENDIF (inverted conditional branch).
func opNotIfScriptPubKey() []byte {
	return []byte{0x64, 0x51, 0x67, 0x00, 0x68}
}

func verifyOPNotIfScript(conditionTrue bool) error {
	var scriptSig []byte
	if conditionTrue {
		scriptSig = []byte{0x51} // OP_1 → NOTIF skips IF branch → ELSE pushes 0
	} else {
		scriptSig = []byte{0x00} // OP_0 → NOTIF takes IF branch → pushes 1
	}
	return scriptErrorToGo(VerifyScriptTest(scriptSig, opNotIfScriptPubKey(), ScriptVerifyMinimalData))
}

func verifyOPVerifyScript(wantTrue bool) error {
	var pub []byte
	if wantTrue {
		pub = []byte{0x51, 0x76, 0x69} // OP_1 OP_DUP OP_VERIFY
	} else {
		pub = []byte{0x00, 0x76, 0x69} // OP_0 OP_DUP OP_VERIFY
	}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPToAltStackRoundtrip() error {
	// OP_1 OP_TOALTSTACK OP_FROMALTSTACK - alt stack round-trip leaves true on main stack.
	pub := []byte{0x51, 0x6b, 0x6c}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPFromAltStackEmpty() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x6c}, ScriptVerifyMinimalData))
}

func verifyOPPickScript() error {
	// OP_0 OP_1 OP_2 OP_1 OP_PICK OP_VERIFY - pick depth 1 yields OP_1.
	pub := []byte{0x00, 0x51, 0x52, 0x51, 0x79, 0x69}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPDepthScript() error {
	// OP_1 OP_DEPTH OP_1 OP_NUMEQUAL - stack depth is 1.
	pub := []byte{0x51, 0x74, 0x51, 0x9c}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPDropEmptyScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x75}, ScriptVerifyMinimalData))
}

func verifyOPRollScript() error {
	// OP_0 OP_1 OP_2 OP_1 OP_ROLL OP_VERIFY - roll depth 1 moves OP_2 to top.
	pub := []byte{0x00, 0x51, 0x52, 0x51, 0x7a, 0x69}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPRotScript() error {
	// OP_1 OP_2 OP_3 OP_ROT OP_VERIFY - rotate top three; OP_1 ends on top.
	pub := []byte{0x51, 0x52, 0x53, 0x7b, 0x69}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPNestedIfScript(innerTrue bool) error {
	var scriptSig []byte
	if innerTrue {
		scriptSig = []byte{0x51, 0x51} // outer IF true, inner IF true
	} else {
		scriptSig = []byte{0x51, 0x00} // outer IF true, inner IF false → inner ELSE pushes 0
	}
	pub := []byte{0x63, 0x63, 0x51, 0x67, 0x00, 0x68, 0x67, 0x00, 0x68} // IF IF OP_1 ELSE OP_0 ENDIF ELSE OP_0 ENDIF
	return scriptErrorToGo(VerifyScriptTest(scriptSig, pub, ScriptVerifyMinimalData))
}

func verifyOPDisabledCatScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x7e}, ScriptVerifyMinimalData))
}

func verifyOPOverScript() error {
	// OP_1 OP_2 OP_OVER OP_VERIFY - copies second-from-top (OP_1).
	pub := []byte{0x51, 0x52, 0x78, 0x69}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOP2DupScript() error {
	// OP_1 OP_2 OP_2DUP OP_VERIFY - duplicates top two; OP_1 ends on top.
	pub := []byte{0x51, 0x52, 0x6e, 0x69}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPIfDupScript() error {
	// OP_1 OP_IFDUP OP_VERIFY - duplicates truthy top element.
	pub := []byte{0x51, 0x73, 0x69}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPUnbalancedIfScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x63}, ScriptVerifyMinimalData))
}

func verifyOPSwapScript() error {
	// OP_1 OP_2 OP_SWAP OP_VERIFY - swap top two; OP_1 ends on top.
	pub := []byte{0x51, 0x52, 0x7c, 0x69}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPTuckScript() error {
	// OP_1 OP_2 OP_TUCK OP_VERIFY - tuck inserts copy of top under second item; OP_2 on top.
	pub := []byte{0x51, 0x52, 0x7d, 0x69}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPNipScript() error {
	// OP_1 OP_1 OP_NIP - nip removes second-from-top; OP_1 remains as final true.
	pub := []byte{0x51, 0x51, 0x77}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOP3DupScript() error {
	// OP_1 OP_2 OP_3 OP_3DUP OP_VERIFY - duplicate top three; OP_1 ends on top.
	pub := []byte{0x51, 0x52, 0x53, 0x6f, 0x69}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPEqualScript(equal bool) error {
	if equal {
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x87}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x52, 0x87}, ScriptVerifyMinimalData))
}

func verifyOPSizeScript() error {
	// PUSH2 0x0102 OP_SIZE OP_2 OP_EQUAL - pushed blob length is 2.
	pub := []byte{0x02, 0x01, 0x02, 0x82, 0x52, 0x87}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOP2OverScript() error {
	// OP_1..OP_4 OP_2OVER - copies bottom pair to top; OP_1 ends on stack.
	pub := []byte{0x51, 0x52, 0x53, 0x54, 0x70}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPBoolAndScript(bothTrue bool) error {
	if bothTrue {
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x9a}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x00, 0x9a}, ScriptVerifyMinimalData))
}

func verifyOPBoolOrScript(wantTrue bool) error {
	if wantTrue {
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x00, 0x51, 0x9b}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x00, 0x00, 0x9b}, ScriptVerifyMinimalData))
}

func verifyOPNumEqualScript(equal bool) error {
	if equal {
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x52, 0x9c}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x52, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP2SwapScript() error {
	// OP_1..OP_4 OP_2SWAP - swap top two pairs; OP_2 ends on stack.
	pub := []byte{0x51, 0x52, 0x53, 0x54, 0x72}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPNotScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x00, 0x91}, ScriptVerifyMinimalData))
}

func verifyOP1AddScript() error {
	// OP_1 OP_1ADD OP_2 OP_NUMEQUAL - 1+1 == 2.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x8b, 0x52, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP2RotScript() error {
	// OP_1..OP_6 OP_2ROT - rotate sixth/seventh from top; OP_2 ends on stack.
	pub := []byte{0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x71}
	return scriptErrorToGo(VerifyScriptTest(nil, pub, ScriptVerifyMinimalData))
}

func verifyOPAddScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x52, 0x93, 0x53, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOPNegateScript() error {
	// OP_2 OP_NEGATE OP_1 OP_NUMNOTEQUAL - -2 != 1.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x8f, 0x51, 0x9e}, ScriptVerifyMinimalData))
}

func verifyOPReturnScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x6a}, ScriptVerifyMinimalData))
}

func verifyOPSubScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x53, 0x51, 0x94, 0x52, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP1SubScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x8c, 0x51, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOPLessThanScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x52, 0x9f}, ScriptVerifyMinimalData))
}

func verifyOPGreaterThanScript(wantTrue bool) error {
	if wantTrue {
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x51, 0xa0}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x52, 0xa0}, ScriptVerifyMinimalData))
}

func verifyOPLessThanOrEqualScript(wantTrue bool) error {
	if wantTrue {
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x52, 0xa1}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x53, 0x52, 0xa1}, ScriptVerifyMinimalData))
}

func verifyOPGreaterThanOrEqualScript(wantTrue bool) error {
	if wantTrue {
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x51, 0xa2}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x52, 0xa2}, ScriptVerifyMinimalData))
}

func verifyOPWithinScript(wantTrue bool) error {
	if wantTrue {
		// OP_2 OP_1 OP_3 OP_WITHIN - 2 is within [1, 3).
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x51, 0x53, 0xa5}, ScriptVerifyMinimalData))
	}
	// OP_3 OP_1 OP_2 OP_WITHIN - 3 is not within [1, 2).
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x53, 0x51, 0x52, 0xa5}, ScriptVerifyMinimalData))
}

func verifyOPDisabledMulScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x97}, ScriptVerifyMinimalData))
}

func verifyOPDisabledDivScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x96}, ScriptVerifyMinimalData))
}

func verifyOPMinScript() error {
	// OP_3 OP_1 OP_MIN OP_1 OP_NUMEQUAL - min(3, 1) == 1.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x53, 0x51, 0xa3, 0x51, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOPMaxScript() error {
	// OP_1 OP_3 OP_MAX OP_3 OP_NUMEQUAL - max(1, 3) == 3.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x53, 0xa4, 0x53, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOPNumNotEqualScript(wantTrue bool) error {
	if wantTrue {
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x52, 0x9e}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x52, 0x9e}, ScriptVerifyMinimalData))
}

func verifyOPDisabledModScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x95}, ScriptVerifyMinimalData))
}

func verifyOPCodeSeparatorScript() error {
	pub, err := ParseScriptASM("NOP CODESEPARATOR 1")
	if err != nil {
		return err
	}
	return scriptErrorToGo(VerifyScriptTest([]byte{0x61}, pub, 0))
}

func verifyOPNumEqualVerifyScript(wantTrue bool) error {
	if wantTrue {
		// OP_2 OP_2 NUMEQUALVERIFY OP_1 - equalverify consumes pair; OP_1 remains true.
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x52, 0x9d, 0x51}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x52, 0x9d}, ScriptVerifyMinimalData))
}

func verifyOPEqualVerifyScript(wantTrue bool) error {
	if wantTrue {
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x88, 0x51}, ScriptVerifyMinimalData))
	}
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x52, 0x88}, ScriptVerifyMinimalData))
}

func verifyOPDisabledLShiftScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x98}, ScriptVerifyMinimalData))
}

func verifyOPDisabledRShiftScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x99}, ScriptVerifyMinimalData))
}

func verifyOPDisabled2MulScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x8d}, ScriptVerifyMinimalData))
}

func verifyOPDisabledAndScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x84}, ScriptVerifyMinimalData))
}

func verifyOPDisabledOrScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x85}, ScriptVerifyMinimalData))
}

func verifyOPDisabledXorScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x86}, ScriptVerifyMinimalData))
}

func verifyOPDisabled2DivScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x8e}, ScriptVerifyMinimalData))
}

func verifyOPDropScript() error {
	// OP_1 OP_1 OP_DROP - drop removes second item; OP_1 remains true.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x75}, ScriptVerifyMinimalData))
}

func verifyOPDupScript() error {
	// OP_1 OP_DUP OP_EQUAL - duplicate top element.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x76, 0x51, 0x87}, ScriptVerifyMinimalData))
}

func verifyOPDisabledLeftScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x80}, ScriptVerifyMinimalData))
}

func verifyOPDisabledRightScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x81}, ScriptVerifyMinimalData))
}

func verifyOPDisabledInvertScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x83}, ScriptVerifyMinimalData))
}

func verifyOPDisabledSubstrScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x7f}, ScriptVerifyMinimalData))
}

func verifyOP2DropScript() error {
	// OP_1 OP_1 OP_1 OP_2DROP - removes two items; OP_1 remains true.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x51, 0x6d}, ScriptVerifyMinimalData))
}

func verifyOPAbsScript() error {
	// OP_1 OP_NEGATE OP_ABS OP_1 OP_NUMEQUAL - abs(-1) == 1.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x8f, 0x90, 0x51, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP0NotEqualScript(wantTrue bool) error {
	if wantTrue {
		// OP_1 OP_0NOTEQUAL - 1 is not equal to 0.
		return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x92}, ScriptVerifyMinimalData))
	}
	// OP_0 OP_0NOTEQUAL - 0 equals 0; stack is false.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x00, 0x92}, ScriptVerifyMinimalData))
}

func verifyOPReservedScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x50}, ScriptVerifyMinimalData))
}

func verifyOPNopScript() error {
	// OP_1 OP_NOP - NOP is a no-op; OP_1 remains true.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x61}, ScriptVerifyMinimalData))
}

func verifyOP1NegateScript() error {
	// OP_1NEGATE OP_NEGATE OP_1 OP_NUMEQUAL - negate(-1) == 1.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x4f, 0x8f, 0x51, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOPVerScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x62}, ScriptVerifyMinimalData))
}

func verifyOPReserved1Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x89}, ScriptVerifyMinimalData))
}

func verifyOPReserved2Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x8a}, ScriptVerifyMinimalData))
}

func verifyOPElseUnbalancedScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x67}, ScriptVerifyMinimalData))
}

func verifyOPEndifUnbalancedScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x68}, ScriptVerifyMinimalData))
}

func verifyOP16Script() error {
	// OP_16 OP_16 OP_NUMEQUAL
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x60, 0x60, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP2Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x52, 0x52, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP15Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x5f, 0x5f, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOPIfEmptyStackScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x63}, ScriptVerifyMinimalData))
}

func verifyOPVerifyEmptyStackScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x69}, ScriptVerifyMinimalData))
}

func verifyOPPickUnderflowScript() error {
	// OP_1 OP_PICK - needs depth 2+ on stack for pick(1) from single item.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x79}, ScriptVerifyMinimalData))
}

func verifyOP3Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x53, 0x53, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP10Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x5a, 0x5a, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOPNotifEmptyStackScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x64}, ScriptVerifyMinimalData))
}

func verifyOPRollUnderflowScript() error {
	// OP_1 OP_ROLL - roll(1) needs at least two stack items.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x7a}, ScriptVerifyMinimalData))
}

func verifyOPDepthEmptyScript() error {
	// OP_DEPTH on empty stack pushes 0, which is false.
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x74}, ScriptVerifyMinimalData))
}

func verifyOP4Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x54, 0x54, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP5Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x55, 0x55, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP6Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x56, 0x56, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP7Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x57, 0x57, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOP8Script() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x58, 0x58, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOPEqualVerifyEmptyScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x88}, ScriptVerifyMinimalData))
}

func verifyOPNumEqualVerifyEmptyScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x9d}, ScriptVerifyMinimalData))
}

func verifyOPSmallIntEqualScript(n int) error {
	if n < 1 || n > 16 {
		return fmt.Errorf("op small int out of range: %d", n)
	}
	op := byte(0x50 + n)
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{op, op, 0x9c}, ScriptVerifyMinimalData))
}

func verifyOPOverUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x78}, ScriptVerifyMinimalData))
}

func verifyOPTuckUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x77}, ScriptVerifyMinimalData))
}

func verifyOPRotUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x7b}, ScriptVerifyMinimalData))
}

func verifyOP2DropUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x6d}, ScriptVerifyMinimalData))
}

func verifyOP2SwapUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x72}, ScriptVerifyMinimalData))
}

func verifyOPDupEmptyScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x76}, ScriptVerifyMinimalData))
}

func verifyOPSwapUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x7c}, ScriptVerifyMinimalData))
}

func verifyOPNipUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x77}, ScriptVerifyMinimalData))
}

func verifyOPIfDupEmptyScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x73}, ScriptVerifyMinimalData))
}

func verifyOP2OverUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x70}, ScriptVerifyMinimalData))
}

func verifyOP3DupUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x6f}, ScriptVerifyMinimalData))
}

func verifyOP2RotUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x51, 0x51, 0x71}, ScriptVerifyMinimalData))
}

func verifyOP2DupUnderflowScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x51, 0x6e}, ScriptVerifyMinimalData))
}

func verifyOPToAltStackEmptyScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x6b}, ScriptVerifyMinimalData))
}

func verifyOP2DropEmptyScript() error {
	return scriptErrorToGo(VerifyScriptTest(nil, []byte{0x6d}, ScriptVerifyMinimalData))
}
