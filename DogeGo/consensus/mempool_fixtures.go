// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/json"
	"fmt"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

// MempoolDifferentialVector is one row from testdata/core_mempool_vectors.json.
type MempoolDifferentialVector struct {
	Name             string `json:"name"`
	Template         string `json:"template"`
	WantAccept       bool   `json:"want_accept"`
	WantRejectReason string `json:"want_reject_reason"`
}

// MempoolDifferentialFixture is a raw transaction plus optional pool setup for RPC/harness parity.
type MempoolDifferentialFixture struct {
	Raw      []byte
	Prep     func(pool *mempool.Pool) error
	View     PrevOutView
	Index    TxIndexer
	Journal  HeaderChain
	Net      chain.Network // zero defaults to RebootTestnet in harness/RPC tests
	Standard *StandardPolicy
}

// LoadMempoolDifferentialVectors reads the Core-shaped mempool policy corpus.
func LoadMempoolDifferentialVectors() ([]MempoolDifferentialVector, error) {
	raw, err := readConsensusTestdata("core_mempool_vectors.json", embeddedCoreMempoolVectorsJSON)
	if err != nil {
		return nil, err
	}
	var vecs []MempoolDifferentialVector
	if err := json.Unmarshal(raw, &vecs); err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("no mempool vectors")
	}
	return vecs, nil
}

// BuildMempoolDifferentialFixture constructs the transaction under test and any mempool prep hooks.
func BuildMempoolDifferentialFixture(template string) (MempoolDifferentialFixture, error) {
	switch template {
	case "coinbase":
		tx, err := minimalCoinbaseTx()
		if err != nil {
			return MempoolDifferentialFixture{}, err
		}
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
	case "duplicate_vin":
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{
				{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff},
				{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff},
			},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
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
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
	case "p2pkh_roundtrip":
		return buildP2PKHRoundtripFixture()
	case "package_ancestor_limit":
		return buildPackageAncestorLimitFixture()
	case "package_descendant_limit":
		return buildPackageDescendantLimitFixture()
	case "package_ancestor_size":
		return buildPackageAncestorSizeFixture()
	case "package_descendant_size":
		return buildPackageDescendantSizeFixture()
	case "mempool_double_spend":
		return buildMempoolDoubleSpendFixture()
	case "min_relay_fee":
		return buildMinRelayFeeFixture()
	case "rbf_insufficient_fee":
		return buildRBFInsufficientFeeFixture()
	case "rbf_sufficient_fee":
		return buildRBFSufficientFeeFixture()
	case "rbf_not_replaceable":
		return buildRBFNotReplaceableFixture()
	case "rbf_fullrbf":
		return buildRBFFullRBFAcceptFixture()
	case "rbf_too_many_descendants":
		return buildRBFTTooManyDescendantsRejectFixture()
	case "rbf_too_many_conflicts":
		return buildRBFTooManyConflictsRejectFixture()
	case "rbf_new_unconfirmed_input":
		return buildRBFNewUnconfirmedInputRejectFixture()
	case "non_bip68_final":
		return buildNonBIP68FinalRejectFixture()
	case "coinbase_immature":
		return buildCoinbaseImmatureFixture()
	case "p2sh_nested_p2pkh":
		return buildP2SHNestedP2PKHFixture()
	case "p2sh_multisig":
		return buildP2SHMultisigFixture()
	case "bare_multisig":
		return buildBareMultisigFixture()
	case "p2sh_cltv_p2pk":
		return buildP2SHCLTVP2PKFixture()
	case "p2sh_csv_p2pk":
		return buildP2SHCSVP2PKFixture()
	case "p2pk_non_standard_input":
		return buildP2PKNonStandardInputFixture()
	case "dust_output_reject":
		return buildDustOutputRejectFixture()
	case "witness_reject":
		return buildWitnessRejectFixture()
	case "bare_multisig_output_disabled":
		return buildBareMultisigOutputDisabledFixture()
	case "op_return_nonzero_reject":
		return buildOpReturnNonZeroRejectFixture()
	case "vout_empty":
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{2}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    nil,
		}
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
	case "vout_negative":
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: -1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
	case "vin_empty":
		tx := &wire.Tx{
			Version: 1,
			Vin:     nil,
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
	case "vout_toolarge":
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{4}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: MaxMoney + 1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
	case "prevout_null":
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{
				{PrevHash: [32]byte{5}, PrevIdx: 0, Sequence: 0xffffffff},
				{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff, Script: []byte{0x01}},
			},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
	case "vout_empty_scriptpubkey":
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{6}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: nil}},
		}
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
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
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
	case "tx_oversize":
		bigScript := make([]byte, MaxBlockBaseSize+10)
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{8}, PrevIdx: 0, Sequence: 0xffffffff, Script: bigScript}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		return MempoolDifferentialFixture{Raw: raw}, err
	case "unspendable_output":
		return buildUnspendableOutputRejectFixture()
	case "op_return_zero":
		return buildOpReturnZeroAcceptFixture()
	case "pq_commitment_op_return":
		return buildPqCommitmentOpReturnAcceptFixture()
	case "pq_commitment_nonzero_reject":
		return buildPqCommitmentNonzeroRejectFixture()
	case "pq_carrier_p2sh_accept":
		return buildPqCarrierP2SHAcceptFixture()
	case "absurd_fee":
		return buildAbsurdFeeRejectFixture()
	case "multi_op_return":
		return buildMultiOpReturnRejectFixture()
	case "tx_version_nonstandard":
		return buildTxVersionNonstandardRejectFixture()
	case "scriptsig_not_pushonly":
		return buildScriptSigNotPushonlyRejectFixture()
	case "non_final":
		return buildNonFinalRejectFixture()
	case "tx_size_small_reject":
		return buildTxSizeSmallRejectFixture()
	case "scriptsig_size_reject":
		return buildScriptSigSizeRejectFixture()
	case "discourage_nop_reject":
		return buildDiscourageNopRejectFixture()
	case "op_return_oversize_reject":
		return buildOpReturnOversizeRejectFixture()
	case "p2sh_sigops_reject":
		return buildP2SHSigopsRejectFixture()
	case "non_standard_output_reject":
		return buildNonStandardOutputRejectFixture()
	case "datacarrier_disabled_reject":
		return buildDatacarrierDisabledRejectFixture()
	case "p2sh_redeem_missing_reject":
		return buildP2SHRedeemMissingRejectFixture()
	case "discourage_nop1_reject":
		return buildDiscourageNop1RejectFixture()
	case "rbf_too_many_descendants_reject":
		return buildRBFTTooManyDescendantsRejectFixture()
	case "tx_version_zero_reject":
		return buildTxVersionZeroRejectFixture()
	case "discourage_nop6_reject":
		return buildDiscourageNop6RejectFixture()
	case "non_bip68_final_reject":
		return buildNonBIP68FinalRejectFixture()
	default:
		return MempoolDifferentialFixture{}, fmt.Errorf("unknown mempool template %q", template)
	}
}

func FixtureNetwork(n chain.Network) chain.Network {
	if n == 0 {
		return chain.RebootTestnet
	}
	return n
}

func buildP2PKHRoundtripFixture() (MempoolDifferentialFixture, error) {
	spend, adm, err := buildP2PKHMempoolRoundTrip()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	mp, ok := adm.Pool.(*mempool.Pool)
	if !ok {
		return MempoolDifferentialFixture{}, fmt.Errorf("p2pkh_roundtrip: expected *mempool.Pool")
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(mp, p)
		},
	}, nil
}

func buildPackageAncestorLimitFixture() (MempoolDifferentialFixture, error) {
	pool := mempool.New(100)
	var prevHash [32]byte
	prevHash[0] = 0xaa
	for i := 0; i < 26; i++ {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: fixtureP2PKHScript()}},
		}
		if err := pool.Add(parent.SerializeForHash()); err != nil {
			return MempoolDifferentialFixture{}, err
		}
		prevHash = parent.TxHash()
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: fixtureChildOutValue(), PkScript: fixtureP2PKHScript()}},
	}
	raw, err := child.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
	}, nil
}

func buildPackageDescendantLimitFixture() (MempoolDifferentialFixture, error) {
	pool := mempool.New(100)
	var prevHash [32]byte
	prevHash[0] = 0xaa
	for i := 0; i < 25; i++ {
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: fixtureP2PKHScript()}},
		}
		if err := pool.Add(tx.SerializeForHash()); err != nil {
			return MempoolDifferentialFixture{}, err
		}
		prevHash = tx.TxHash()
	}
	extra := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: fixtureChildOutValue(), PkScript: fixtureP2PKHScript()}},
	}
	raw, err := extra.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
	}, nil
}

func buildPackageAncestorSizeFixture() (MempoolDifferentialFixture, error) {
	pool := mempool.New(100)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff, Script: make([]byte, 900)}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: fixtureP2PKHScript()}},
	}
	if err := pool.Add(parent.SerializeForHash()); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 49_000_000, PkScript: fixtureP2PKHScript()}},
	}
	raw, err := child.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
	}, nil
}

func buildPackageDescendantSizeFixture() (MempoolDifferentialFixture, error) {
	pool := mempool.New(100)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: fixtureP2PKHScript()}},
	}
	parentRaw, err := parent.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(parentRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	child1 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff, Script: make([]byte, 900)}},
		Vout:    []wire.TxOut{{Value: 49_000_000, PkScript: fixtureP2PKHScript()}},
	}
	if err := pool.Add(child1.SerializeForHash()); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	child2 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: child1.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 48_000_000, PkScript: fixtureP2PKHScript()}},
	}
	raw, err := child2.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	parentID := txidDisplayFromLE([32]byte{9})
	c1h := child1.TxHash()
	view := fixedPrevOutView{
		rpcOutpointKey(parentID, 0): {Value: parent.Vout[0].Value, PkScript: parent.Vout[0].PkScript},
		rpcOutpointKey(txidDisplayFromLE(c1h), 0): {Value: child1.Vout[0].Value, PkScript: child1.Vout[0].PkScript},
	}
	return MempoolDifferentialFixture{
		Raw:  raw,
		Prep: func(p *mempool.Pool) error { return copyPoolEntries(pool, p) },
		View: view,
	}, nil
}

func buildRBFInsufficientFeeFixture() (MempoolDifferentialFixture, error) {
	raw, prep, view, err := BuildRBFInsufficientFeeDifferentialFixture()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw:  raw,
		Prep: prep,
		View: view,
	}, nil
}

func buildRBFSufficientFeeFixture() (MempoolDifferentialFixture, error) {
	raw, prep, view, err := BuildRBFSufficientFeeDifferentialFixture()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw:  raw,
		Prep: prep,
		View: view,
	}, nil
}

func buildRBFNotReplaceableFixture() (MempoolDifferentialFixture, error) {
	raw, prep, view, err := BuildRBFNotReplaceableDifferentialFixture()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw:  raw,
		Prep: prep,
		View: view,
	}, nil
}

func buildRBFFullRBFAcceptFixture() (MempoolDifferentialFixture, error) {
	raw, prep, view, err := BuildRBFFullRBFAcceptFixture()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw:  raw,
		Prep: prep,
		View: view,
	}, nil
}

func buildP2SHMultisigFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x77
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildFixtureMultisigRedeem(1, pubC)
	p2sh := fixtureP2SHScript(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xab}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	h160 := hash160(pubC)
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	digest, err := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildFixtureMultisigRedeem(nRequired int, pub []byte) []byte {
	var s []byte
	s = append(s, byte(0x50+nRequired))
	s = append(s, byte(len(pub)))
	s = append(s, pub...)
	s = append(s, 0x51, 0xae)
	return s
}

func buildBareMultisigFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x88
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildFixtureMultisigRedeem(1, pubC)
	h160 := hash160(pubC)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xcd}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: redeem}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
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
	digest, err := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	spend.Vin[0].Script = script.Bytes()
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

const cltvFixtureLockHeight = int64(700)
const cltvFixtureJournalTip = int64(4_000_000)

func buildP2SHCLTVP2PKFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x47
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildCLTVP2PKRedeemScript(cltvFixtureLockHeight, pubC)
	p2sh := fixtureP2SHScript(redeem)
	h160 := hash160(pubC)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{12}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(50)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend := &wire.Tx{
		Version:  1,
		LockTime: uint32(cltvFixtureLockHeight),
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xfffffffe,
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	_, inner, err := parseTimelockDropRedeem(redeem, opCheckLockTimeVerify)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	digest, err := wire.CalcSignatureHashLegacy(inner, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View:    &MempoolPrevOutView{Pool: pool},
		Journal: &DifferentialCLTVJournal{Tip: cltvFixtureJournalTip},
		Net:     chain.MainnetDogecoin,
	}, nil
}

const csvFixtureRelativeSeq = int64(2)
const csvFixtureJournalTip = int64(500_000)

func buildP2SHCSVP2PKFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x55
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildCSVP2PKRedeemScript(csvFixtureRelativeSeq, pubC)
	p2sh := fixtureP2SHScript(redeem)
	h160 := hash160(pubC)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{13}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(50)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: uint32(csvFixtureRelativeSeq),
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	_, inner, err := parseTimelockDropRedeem(redeem, opCheckSequenceVerify)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	digest, err := wire.CalcSignatureHashLegacy(inner, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View:    &MempoolPrevOutView{Pool: pool},
		Journal: &DifferentialCLTVJournal{Tip: csvFixtureJournalTip},
		Net:     chain.MainnetDogecoin,
	}, nil
}

func buildP2PKNonStandardInputFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x42
	_, pub := secp256k1.PrivKeyFromBytes(sec)
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
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
			Script:   []byte{0x51},
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildDustOutputRejectFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0xdd
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := standardP2PKHScript(h160[:])

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xdd}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000, PkScript: pkScript}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: fixtureP2PKHScript()}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildWitnessRejectFixture() (MempoolDifferentialFixture, error) {
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{1},
			PrevIdx:  0,
			Sequence: 0xffffffff,
			Witness:  [][]byte{{0x01}},
		}},
		Vout: []wire.TxOut{{Value: HardDustLimitKoinu, PkScript: fixtureP2PKHScript()}},
	}
	raw, err := tx.Serialize()
	return MempoolDifferentialFixture{Raw: raw}, err
}

func buildBareMultisigOutputDisabledFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x89
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := buildFixtureMultisigRedeem(1, pubC)
	h160 := hash160(pubC)
	pkScript := standardP2PKHScript(h160[:])

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xce}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: pkScript}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 4_000_000_000, PkScript: redeem}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	pol := DefaultStandardPolicy()
	pol.AllowBareMultisig = false
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View:     &MempoolPrevOutView{Pool: pool},
		Standard: &pol,
	}, nil
}

func buildOpReturnNonZeroRejectFixture() (MempoolDifferentialFixture, error) {
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xef}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000, PkScript: fixtureP2PKHScript()}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x6a, 0x04, 0xde, 0xad, 0xbe, 0xef}}},
	}
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildUnspendableOutputRejectFixture() (MempoolDifferentialFixture, error) {
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xf0}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000, PkScript: fixtureP2PKHScript()}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x6a, 0x04, 0xca, 0xfe, 0xba, 0xbe}}},
	}
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildOpReturnZeroAcceptFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x48
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xf1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: pkScript}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{
			{Value: 99_000_000, PkScript: standardP2PKHScript(h160[:])},
			{Value: 0, PkScript: []byte{0x6a, 0x00}},
		},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)

	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildAbsurdFeeRejectFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x49
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	inVal := DefaultMaxAbsurdTxFeeKoinu + 1_000_000
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xf2}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: inVal, PkScript: pkScript}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: inVal - DefaultMaxAbsurdTxFeeKoinu - 1000, PkScript: standardP2PKHScript(h160[:])}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)

	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildMultiOpReturnRejectFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x4a
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xf3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: pkScript}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{
			{Value: 99_000_000, PkScript: standardP2PKHScript(h160[:])},
			{Value: 0, PkScript: []byte{0x6a, 0x00}},
			{Value: 0, PkScript: []byte{0x6a, 0x00}},
		},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)

	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildTxVersionNonstandardRejectFixture() (MempoolDifferentialFixture, error) {
	pad := make([]byte, 24)
	tx := &wire.Tx{
		Version: 3,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xf5}, PrevIdx: 0, Sequence: 0xffffffff, Script: pad}},
		Vout:    []wire.TxOut{{Value: HardDustLimitKoinu + 1_000_000, PkScript: fixtureP2PKHScript()}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{Raw: raw}, nil
}

func buildScriptSigNotPushonlyRejectFixture() (MempoolDifferentialFixture, error) {
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{0xf6},
			PrevIdx:  0,
			Sequence: 0xffffffff,
			Script:   []byte{0x51, 0xac},
		}},
		Vout: []wire.TxOut{{Value: HardDustLimitKoinu + 1_000_000, PkScript: fixtureP2PKHScript()}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{Raw: raw}, nil
}

func buildNonFinalRejectFixture() (MempoolDifferentialFixture, error) {
	spend, j, view := NonFinalDifferentialSpend()
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw:     raw,
		Journal: j,
		View:    view,
	}, nil
}

func buildTxSizeSmallRejectFixture() (MempoolDifferentialFixture, error) {
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{0xf7},
			PrevIdx:  0,
			Script:   []byte{},
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{
			Value:    0,
			PkScript: []byte{0x6a, 0x00},
		}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if len(raw) >= MinStandardTxNonWitnessSize {
		return MempoolDifferentialFixture{}, fmt.Errorf("tx_size_small fixture too large: %d bytes", len(raw))
	}
	return MempoolDifferentialFixture{Raw: raw}, nil
}

func buildScriptSigSizeRejectFixture() (MempoolDifferentialFixture, error) {
	bigSig := make([]byte, MaxStandardScriptSig+1)
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{0xf8},
			PrevIdx:  0,
			Sequence: 0xffffffff,
			Script:   bigSig,
		}},
		Vout: []wire.TxOut{{Value: HardDustLimitKoinu + 1_000_000, PkScript: fixtureP2PKHScript()}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{Raw: raw}, nil
}

func buildDiscourageNopRejectFixture() (MempoolDifferentialFixture, error) {
	redeem := []byte{0xb4, 0x51} // OP_NOP4 OP_1
	p2sh := fixtureP2SHScript(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xf9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: fixtureP2PKHScript()}},
	}
	var script bytes.Buffer
	script.WriteByte(0x00) // OP_0 dummy push so redeem is scanned for discouraged ops
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()

	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildOpReturnOversizeRejectFixture() (MempoolDifferentialFixture, error) {
	pkScript := make([]byte, MaxDatacarrierBytes+1)
	pkScript[0] = 0x6a
	pkScript[1] = 0x4c
	pkScript[2] = byte(MaxDatacarrierBytes - 2)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xfa}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 0, PkScript: pkScript}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{Raw: raw}, nil
}

func buildP2SHSigopsRejectFixture() (MempoolDifferentialFixture, error) {
	redeem := make([]byte, 0, 32)
	for i := 0; i < 16; i++ {
		redeem = append(redeem, 0xac)
	}
	p2sh := fixtureP2SHScript(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xfb}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: fixtureP2PKHScript()}},
	}
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()

	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildNonStandardOutputRejectFixture() (MempoolDifferentialFixture, error) {
	pad := make([]byte, 24)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xfc}, PrevIdx: 0, Sequence: 0xffffffff, Script: pad}},
		Vout:    []wire.TxOut{{Value: HardDustLimitKoinu + 1_000_000, PkScript: []byte{0x99, 0x88}}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if len(raw) < MinStandardTxNonWitnessSize {
		return MempoolDifferentialFixture{}, fmt.Errorf("non_standard_output fixture too small: %d bytes", len(raw))
	}
	return MempoolDifferentialFixture{Raw: raw}, nil
}

func buildDatacarrierDisabledRejectFixture() (MempoolDifferentialFixture, error) {
	pad := make([]byte, 24)
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xfd}, PrevIdx: 0, Sequence: 0xffffffff, Script: pad}},
		Vout:    []wire.TxOut{{Value: 0, PkScript: []byte{0x6a, 0x00}}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if len(raw) < MinStandardTxNonWitnessSize {
		return MempoolDifferentialFixture{}, fmt.Errorf("datacarrier_disabled fixture too small: %d bytes", len(raw))
	}
	pol := DefaultStandardPolicy()
	pol.AcceptDataCarrier = false
	return MempoolDifferentialFixture{Raw: raw, Standard: &pol}, nil
}

func buildP2SHRedeemMissingRejectFixture() (MempoolDifferentialFixture, error) {
	redeem := []byte{0x51}
	p2sh := fixtureP2SHScript(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xfe}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
			Script:   []byte{0x00},
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: fixtureP2PKHScript()}},
	}
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildDiscourageNop1RejectFixture() (MempoolDifferentialFixture, error) {
	redeem := []byte{0xb0, 0x51} // OP_NOP1 OP_1
	p2sh := fixtureP2SHScript(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xff}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: fixtureP2PKHScript()}},
	}
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()

	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildRBFTTooManyDescendantsRejectFixture() (MempoolDifferentialFixture, error) {
	raw, prep, view, err := BuildRBFTTooManyDescendantsFixture()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{Raw: raw, Prep: prep, View: view}, nil
}

func buildRBFTooManyConflictsRejectFixture() (MempoolDifferentialFixture, error) {
	raw, prep, view, err := BuildRBFTooManyConflictsFixture()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{Raw: raw, Prep: prep, View: view}, nil
}

func buildRBFNewUnconfirmedInputRejectFixture() (MempoolDifferentialFixture, error) {
	raw, prep, view, err := BuildRBFNewUnconfirmedInputFixture()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{Raw: raw, Prep: prep, View: view}, nil
}

func buildTxVersionZeroRejectFixture() (MempoolDifferentialFixture, error) {
	pad := make([]byte, 24)
	tx := &wire.Tx{
		Version: 0,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0x01}, PrevIdx: 0, Sequence: 0xffffffff, Script: pad}},
		Vout:    []wire.TxOut{{Value: HardDustLimitKoinu + 1_000_000, PkScript: fixtureP2PKHScript()}},
	}
	raw, err := tx.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{Raw: raw}, nil
}

func buildDiscourageNop6RejectFixture() (MempoolDifferentialFixture, error) {
	redeem := []byte{0xb6, 0x51} // OP_NOP6 OP_1
	p2sh := fixtureP2SHScript(redeem)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0x02}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: fixtureP2PKHScript()}},
	}
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()

	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildNonBIP68FinalRejectFixture() (MempoolDifferentialFixture, error) {
	spend, j, view := NonBIP68FinalDifferentialSpend()
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw:     raw,
		Journal: j,
		View:    view,
		Net:     chain.MainnetDogecoin,
	}, nil
}

func buildP2SHNestedP2PKHFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x44
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	innerRedeem := standardP2PKHScript(h160[:])
	forward := fixtureP2SHScript(innerRedeem)
	outerP2SH := fixtureP2SHScript(forward)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{11}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 5_000_000_000, PkScript: outerP2SH}},
	}
	pool := mempool.New(50)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
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
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	spend.Vin[0].Script, err = fixtureConcatScriptPushes(sigBytes, pubC, innerRedeem, forward)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func fixtureP2SHScript(redeem []byte) []byte {
	rh := hash160(redeem)
	s := append([]byte{0xa9, 0x14}, rh[:]...)
	return append(s, 0x87)
}

func fixtureConcatScriptPushes(parts ...[]byte) ([]byte, error) {
	var out []byte
	for _, p := range parts {
		if len(p) == 0 {
			out = append(out, 0x00)
			continue
		}
		if len(p) <= 75 {
			out = append(out, byte(len(p)))
			out = append(out, p...)
			continue
		}
		return nil, fmt.Errorf("push too large for fixture helper")
	}
	return out, nil
}

func buildCoinbaseImmatureFixture() (MempoolDifferentialFixture, error) {
	spend, ix, j, view := CoinbaseImmatureDifferentialSpend()
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw:     raw,
		Index:   ix,
		Journal: j,
		View:    view,
	}, nil
}

func buildMinRelayFeeFixture() (MempoolDifferentialFixture, error) {
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 200_000, PkScript: fixtureP2PKHScript()}},
	}
	pool := mempool.New(10)
	if err := pool.Add(parent.SerializeForHash()); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	// Pad scriptsig so the tx meets MIN_STANDARD_TX_NONWITNESS_SIZE but still pays below min relay fee.
	pad := make([]byte, 24)
	child := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: parent.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
			Script:   pad,
		}},
		Vout: []wire.TxOut{{Value: 199_000, PkScript: fixtureP2PKHScript()}},
	}
	raw, err := child.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
	}, nil
}

func buildMempoolDoubleSpendFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x66
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	pool := mempool.New(10)
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend1, err := buildSignedSpendTx(funding, pkScript, priv, pubC, 900_000_000)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(spend1.SerializeForHash()); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend2, err := buildSignedSpendTx(funding, pkScript, priv, pubC, 800_000_000)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	raw, err := spend2.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
	}, nil
}

func copyPoolEntries(from, to *mempool.Pool) error {
	if from == nil || to == nil {
		return nil
	}
	for _, raw := range from.RawBlobs() {
		if err := to.Add(raw); err != nil {
			return err
		}
	}
	return nil
}

func buildSignedSpendTx(funding *wire.Tx, pkScript []byte, priv *secp256k1.PrivateKey, pubC []byte, outVal int64) (*wire.Tx, error) {
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: outVal, PkScript: fixtureP2PKHScript()}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return nil, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	return spend, nil
}

func buildPqCommitmentOpReturnAcceptFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x49
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)
	pqScript, err := BuildPQCommitmentScript(PQTagDilithium, make([]byte, 32))
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xf2}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: pkScript}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{
			{Value: 99_000_000, PkScript: standardP2PKHScript(h160[:])},
			{Value: 0, PkScript: pqScript},
		},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)

	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildPqCommitmentNonzeroRejectFixture() (MempoolDifferentialFixture, error) {
	pqScript, err := BuildPQCommitmentScript(PQTagFalcon, make([]byte, 32))
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xf3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000, PkScript: fixtureP2PKHScript()}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: pqScript}},
	}
	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func buildPqCarrierP2SHAcceptFixture() (MempoolDifferentialFixture, error) {
	sec := make([]byte, 32)
	sec[0] = 0x4a
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)
	carrier := BuildPQCarrierP2SHScriptPubKey()

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{0xf4}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: pkScript}},
	}
	pool := mempool.New(10)
	fundRaw, err := funding.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	if err := pool.Add(fundRaw); err != nil {
		return MempoolDifferentialFixture{}, err
	}

	spend := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{
			{Value: 98_000_000, PkScript: standardP2PKHScript(h160[:])},
			{Value: 1_000_000, PkScript: carrier},
		},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)

	raw, err := spend.Serialize()
	if err != nil {
		return MempoolDifferentialFixture{}, err
	}
	return MempoolDifferentialFixture{
		Raw: raw,
		Prep: func(p *mempool.Pool) error {
			return copyPoolEntries(pool, p)
		},
		View: &MempoolPrevOutView{Pool: pool},
	}, nil
}

func fixtureP2PKHScript() []byte {
	pk := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	return append(pk, 0x88, 0xac)
}

// fixtureChildOutValue is a non-dust P2PKH output for unsigned package-limit fixture txs.
func fixtureChildOutValue() int64 {
	return HardDustLimitKoinu + 1_000_000
}
