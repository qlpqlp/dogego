// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"errors"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

type coreMempoolVector struct {
	Name             string `json:"name"`
	Template         string `json:"template"`
	WantAccept       bool   `json:"want_accept"`
	WantRejectReason string `json:"want_reject_reason"`
}

type mempoolStubPrevOutView map[[36]byte]PrevOut

func (s mempoolStubPrevOutView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	var k [36]byte
	copy(k[:32], prevHash[:])
	k[32] = byte(idx)
	k[33] = byte(idx >> 8)
	k[34] = byte(idx >> 16)
	k[35] = byte(idx >> 24)
	o, ok := s[k]
	return o, ok
}

func buildMempoolAdmissionCase(tmpl string) (*wire.Tx, MempoolAdmission, error) {
	switch tmpl {
	case "coinbase":
		tx, err := minimalCoinbaseTx()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "duplicate_vin":
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{
				{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff},
				{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff},
			},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "missing_prevout":
		pkScript := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
		pkScript = append(pkScript, 0x88, 0xac)
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{{
				PrevHash: [32]byte{1},
				PrevIdx:  0,
				Sequence: 0xffffffff,
			}},
			Vout: []wire.TxOut{{Value: HardDustLimitKoinu, PkScript: pkScript}},
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "p2pkh_roundtrip":
		tx, adm, err := buildP2PKHMempoolRoundTrip()
		return tx, adm, err
	case "p2sh_nested_p2pkh":
		fix, err := buildP2SHNestedP2PKHFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "p2sh_multisig":
		fix, err := buildP2SHMultisigFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "bare_multisig":
		fix, err := buildBareMultisigFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "p2sh_cltv_p2pk":
		fix, err := buildP2SHCLTVP2PKFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "p2sh_csv_p2pk":
		fix, err := buildP2SHCSVP2PKFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "p2pk_non_standard_input":
		fix, err := buildP2PKNonStandardInputFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "dust_output_reject":
		fix, err := buildDustOutputRejectFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "witness_reject":
		fix, err := buildWitnessRejectFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "bare_multisig_output_disabled":
		fix, err := buildBareMultisigOutputDisabledFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "op_return_nonzero_reject":
		fix, err := buildOpReturnNonZeroRejectFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "rbf_sufficient_fee":
		fix, err := buildRBFSufficientFeeFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "rbf_not_replaceable":
		fix, err := buildRBFNotReplaceableFixture()
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	case "vout_empty":
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{2}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    nil,
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "vout_negative":
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: -1, PkScript: []byte{0x51}}},
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "vin_empty":
		tx := &wire.Tx{
			Version: 1,
			Vin:     nil,
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "vout_toolarge":
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{4}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: MaxMoney + 1, PkScript: []byte{0x51}}},
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "prevout_null":
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{
				{PrevHash: [32]byte{5}, PrevIdx: 0, Sequence: 0xffffffff},
				{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff, Script: []byte{0x01}},
			},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "vout_empty_scriptpubkey":
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{6}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: nil}},
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "txouttotal_toolarge":
		half := int64(MaxMoney / 2)
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{7}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout: []wire.TxOut{
				{Value: half + 1, PkScript: []byte{0x51}},
				{Value: half + 1, PkScript: []byte{0x52}},
			},
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "tx_oversize":
		bigScript := make([]byte, MaxBlockBaseSize+10)
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{8}, PrevIdx: 0, Sequence: 0xffffffff, Script: bigScript}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		return tx, MempoolAdmission{View: &MempoolPrevOutView{Pool: mempool.New(10)}}, nil
	case "unspendable_output", "op_return_zero", "pq_commitment_op_return", "pq_commitment_nonzero_reject", "pq_carrier_p2sh_accept", "absurd_fee", "multi_op_return", "tx_version_nonstandard", "scriptsig_not_pushonly", "non_final", "tx_size_small_reject", "scriptsig_size_reject", "discourage_nop_reject", "op_return_oversize_reject", "p2sh_sigops_reject", "non_standard_output_reject", "datacarrier_disabled_reject", "p2sh_redeem_missing_reject", "discourage_nop1_reject", "rbf_too_many_descendants_reject", "tx_version_zero_reject", "discourage_nop6_reject", "non_bip68_final_reject":
		fix, err := BuildMempoolDifferentialFixture(tmpl)
		if err != nil {
			return nil, MempoolAdmission{}, err
		}
		return mempoolAdmissionFromFixture(fix)
	default:
		return nil, MempoolAdmission{}, errors.New("unknown mempool template")
	}
}

func minimalCoinbaseTx() (*wire.Tx, error) {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(&buf, 1)
	var zeros [32]byte
	_, _ = buf.Write(zeros[:])
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_, _ = buf.Write([]byte{0x00})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_ = binary.Write(&buf, binary.LittleEndian, int64(8800000000))
	_ = wire.WriteCompactSize(&buf, 2)
	_, _ = buf.Write([]byte{0x51, 0x51})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	return wire.DeserializeTx(buf.Bytes())
}

func buildP2PKHMempoolRoundTrip() (*wire.Tx, MempoolAdmission, error) {
	sec := make([]byte, 32)
	sec[0] = 0x33
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	fundRaw, err := funding.Serialize()
	if err != nil {
		return nil, MempoolAdmission{}, err
	}
	pool := mempool.New(100)
	if err := pool.Add(fundRaw); err != nil {
		return nil, MempoolAdmission{}, err
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
		return nil, MempoolAdmission{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)

	adm := MempoolAdmission{
		View:             &MempoolPrevOutView{Pool: pool},
		Pool:             pool,
		Net:              chain.RebootTestnet,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
	}
	return spend, adm, nil
}

func mempoolAdmissionFromFixture(fix MempoolDifferentialFixture) (*wire.Tx, MempoolAdmission, error) {
	tx, err := wire.DeserializeTx(fix.Raw)
	if err != nil {
		return nil, MempoolAdmission{}, err
	}
	pool := mempool.New(500)
	if fix.Prep != nil {
		if err := fix.Prep(pool); err != nil {
			return nil, MempoolAdmission{}, err
		}
	}
	view := fix.View
	if view == nil {
		view = &MempoolPrevOutView{Pool: pool}
	}
	adm := MempoolAdmission{
		View:             view,
		Pool:             pool,
		RBFPool:          pool,
		Index:            fix.Index,
		Journal:          fix.Journal,
		Net:              FixtureNetwork(fix.Net),
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
		Standard:         standardPolicyFromFixture(fix),
	}
	return tx, adm, nil
}

func standardPolicyFromFixture(fix MempoolDifferentialFixture) StandardPolicy {
	if fix.Standard != nil {
		return *fix.Standard
	}
	return StandardPolicy{}
}

func standardP2PKHScript(h160 []byte) []byte {
	s := append([]byte{0x76, 0xa9, 0x14}, h160...)
	return append(s, 0x88, 0xac)
}
