// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"dogego/mempool"
	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"
	"dogego/wire"
)

// StatefulLiveProbe is a reboottestnet live mempool probe (prep mempool txs + probe tx).
type StatefulLiveProbe struct {
	Template         string   `json:"template"`
	WantAccept       bool     `json:"want_accept"`
	WantRejectReason string   `json:"want_reject_reason,omitempty"`
	PrepTxHex          []string `json:"prep_tx_hex,omitempty"`
	PrepSubmitBlockHex string   `json:"prep_submit_block_hex,omitempty"`
	ProbeTxHex         string   `json:"probe_tx_hex"`
}

// WalletFundingUTXO is a confirmed wallet output used to anchor a live stateful probe.
type WalletFundingUTXO struct {
	PrevHash [32]byte
	PrevIdx  uint32
	Value    int64
	PkScript []byte
}

// StatefulLiveProbeTemplates are stateful corpus rows with wallet-anchored live builders.
var StatefulLiveProbeTemplates = []string{
	"p2sh_nested_p2pkh",
	"p2sh_multisig",
	"bare_multisig",
	"p2sh_cltv_p2pk",
	"p2sh_csv_p2pk",
	"p2pk_non_standard_input",
	"package_ancestor_size",
	"package_descendant_size",
	"pq_commitment_op_return",
	"pq_carrier_p2sh_accept",
}

// BuildWalletAnchoredStatefulProbe builds signed hex anchored to a real wallet UTXO.
func BuildWalletAnchoredStatefulProbe(template string, priv *secp256k1.PrivateKey, pubCompressed []byte, fund WalletFundingUTXO, chainHeight int64) (StatefulLiveProbe, error) {
	if priv == nil || len(pubCompressed) == 0 {
		return StatefulLiveProbe{}, fmt.Errorf("stateful live probe: missing key")
	}
	if fund.Value < 5_000_000 {
		return StatefulLiveProbe{}, fmt.Errorf("stateful live probe: funding UTXO too small")
	}
	switch template {
	case "p2sh_nested_p2pkh":
		return walletProbeP2SHNested(priv, pubCompressed, fund)
	case "p2sh_multisig":
		return walletProbeP2SHMultisig(priv, pubCompressed, fund)
	case "bare_multisig":
		return walletProbeBareMultisig(priv, pubCompressed, fund)
	case "p2sh_cltv_p2pk":
		return walletProbeP2SHCLTV(priv, pubCompressed, fund, chainHeight)
	case "p2sh_csv_p2pk":
		return walletProbeP2SHCSV(priv, pubCompressed, fund)
	case "p2pk_non_standard_input":
		return walletProbeP2PKNonStandard(priv, pubCompressed, fund)
	case "package_ancestor_size":
		return walletProbePackageAncestorSize(priv, pubCompressed, fund)
	case "package_descendant_size":
		return walletProbePackageDescendantSize(priv, pubCompressed, fund)
	case "pq_commitment_op_return":
		return walletProbePQCommitmentOpReturn(priv, pubCompressed, fund)
	case "pq_carrier_p2sh_accept":
		return walletProbePQCarrierP2SH(priv, pubCompressed, fund)
	default:
		return StatefulLiveProbe{}, fmt.Errorf("unknown stateful live probe template %q", template)
	}
}

// StatefulLiveProbeFromFixture exports offline fixture hex (fictional prevouts; for harness only).
func StatefulLiveProbeFromFixture(template string) (StatefulLiveProbe, error) {
	fix, err := BuildMempoolDifferentialFixture(template)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	vec, err := loadMempoolVector(template)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	var prep []string
	if fix.Prep != nil {
		pool := mempool.New(500)
		_ = fix.Prep(pool)
		for _, raw := range pool.RawBlobs() {
			if bytes.Equal(raw, fix.Raw) {
				continue
			}
			prep = append(prep, hex.EncodeToString(raw))
		}
	}
	return StatefulLiveProbe{
		Template:         template,
		WantAccept:       vec.WantAccept,
		WantRejectReason: vec.WantRejectReason,
		PrepTxHex:        prep,
		ProbeTxHex:       hex.EncodeToString(fix.Raw),
	}, nil
}

func walletProbeP2SHMultisig(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO) (StatefulLiveProbe, error) {
	redeem := buildFixtureMultisigRedeem(1, pubC)
	p2sh := fixtureP2SHScript(redeem)
	h160 := hash160(pubC)
	outVal := fund.Value - 500_000
	anchor, err := signWalletP2PKHSpend(fund, priv, pubC, p2sh, outVal)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: anchor.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: outVal - 500_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	digest, err := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	if err != nil {
		return StatefulLiveProbe{}, err
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
	anchorHex, err := txHex(anchor)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	probeHex, err := txHex(spend)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{Template: "p2sh_multisig", WantAccept: true, PrepTxHex: []string{anchorHex}, ProbeTxHex: probeHex}, nil
}

func walletProbeP2SHNested(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO) (StatefulLiveProbe, error) {
	h160 := hash160(pubC)
	inner := standardP2PKHScript(h160[:])
	forward := fixtureP2SHScript(inner)
	outer := fixtureP2SHScript(forward)
	outVal := fund.Value - 500_000
	anchor, err := signWalletP2PKHSpend(fund, priv, pubC, outer, outVal)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: anchor.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: outVal - 500_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	digest, err := wire.CalcSignatureHashLegacy(inner, wire.SigHashAll, spend, 0)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	spend.Vin[0].Script, err = fixtureConcatScriptPushes(sigBytes, pubC, inner, forward)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	anchorHex, err := txHex(anchor)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	probeHex, err := txHex(spend)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{Template: "p2sh_nested_p2pkh", WantAccept: true, PrepTxHex: []string{anchorHex}, ProbeTxHex: probeHex}, nil
}

func walletProbeBareMultisig(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO) (StatefulLiveProbe, error) {
	redeem := buildFixtureMultisigRedeem(1, pubC)
	h160 := hash160(pubC)
	outVal := fund.Value - 500_000
	anchor, err := signWalletP2PKHSpend(fund, priv, pubC, redeem, outVal)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: anchor.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: outVal - 500_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	digest, err := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(0x00)
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	spend.Vin[0].Script = script.Bytes()
	anchorHex, err := txHex(anchor)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	probeHex, err := txHex(spend)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{Template: "bare_multisig", WantAccept: true, PrepTxHex: []string{anchorHex}, ProbeTxHex: probeHex}, nil
}

func walletProbeP2SHCLTV(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO, chainHeight int64) (StatefulLiveProbe, error) {
	lock := chainHeight
	if lock < 1 {
		lock = cltvFixtureLockHeight
	}
	redeem := buildCLTVP2PKRedeemScript(lock, pubC)
	p2sh := fixtureP2SHScript(redeem)
	h160 := hash160(pubC)
	outVal := fund.Value - 500_000
	anchor, err := signWalletP2PKHSpend(fund, priv, pubC, p2sh, outVal)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	spend := &wire.Tx{
		Version:  1,
		LockTime: uint32(lock),
		Vin: []wire.TxIn{{
			PrevHash: anchor.TxHash(),
			PrevIdx:  0,
			Sequence: 0xfffffffe,
		}},
		Vout: []wire.TxOut{{Value: outVal - 500_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	_, inner, err := parseTimelockDropRedeem(redeem, opCheckLockTimeVerify)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	digest, err := wire.CalcSignatureHashLegacy(inner, wire.SigHashAll, spend, 0)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()
	anchorHex, err := txHex(anchor)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	probeHex, err := txHex(spend)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{Template: "p2sh_cltv_p2pk", WantAccept: true, PrepTxHex: []string{anchorHex}, ProbeTxHex: probeHex}, nil
}

func walletProbeP2SHCSV(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO) (StatefulLiveProbe, error) {
	rel := csvFixtureRelativeSeq
	redeem := buildCSVP2PKRedeemScript(rel, pubC)
	p2sh := fixtureP2SHScript(redeem)
	h160 := hash160(pubC)
	outVal := fund.Value - 500_000
	anchor, err := signWalletP2PKHSpend(fund, priv, pubC, p2sh, outVal)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	spend := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: anchor.TxHash(),
			PrevIdx:  0,
			Sequence: uint32(rel),
		}},
		Vout: []wire.TxOut{{Value: outVal - 500_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	_, inner, err := parseTimelockDropRedeem(redeem, opCheckSequenceVerify)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	digest, err := wire.CalcSignatureHashLegacy(inner, wire.SigHashAll, spend, 0)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()
	anchorHex, err := txHex(anchor)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	probeHex, err := txHex(spend)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{Template: "p2sh_csv_p2pk", WantAccept: true, PrepTxHex: []string{anchorHex}, ProbeTxHex: probeHex}, nil
}

func walletProbeP2PKNonStandard(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO) (StatefulLiveProbe, error) {
	pkScript := append([]byte{0x21}, pubC...)
	pkScript = append(pkScript, 0xac)
	h160 := hash160(pubC)
	outVal := fund.Value - 500_000
	anchor, err := signWalletP2PKHSpend(fund, priv, pubC, pkScript, outVal)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: anchor.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
			Script:   []byte{0x51},
		}},
		Vout: []wire.TxOut{{Value: outVal - 500_000, PkScript: standardP2PKHScript(h160[:])}},
	}
	anchorHex, err := txHex(anchor)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	probeHex, err := txHex(spend)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{
		Template:         "p2pk_non_standard_input",
		WantAccept:       false,
		WantRejectReason: "non-standard-input",
		PrepTxHex:        []string{anchorHex},
		ProbeTxHex:       probeHex,
	}, nil
}

func walletProbePackageAncestorSize(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO) (StatefulLiveProbe, error) {
	h160 := hash160(pubC)
	pkScript := standardP2PKHScript(h160[:])
	outVal := fund.Value - 500_000
	parent, err := signWalletP2PKHSpendWithScriptSig(fund, priv, pubC, pkScript, outVal, make([]byte, 900))
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: outVal - 500_000, PkScript: pkScript}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, child, 0)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	child.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	parentHex, err := txHex(parent)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	childHex, err := txHex(child)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{
		Template:         "package_ancestor_size",
		WantAccept:       false,
		WantRejectReason: "too-long-mempool-chain",
		PrepTxHex:        []string{parentHex},
		ProbeTxHex:       childHex,
	}, nil
}

func walletProbePackageDescendantSize(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO) (StatefulLiveProbe, error) {
	h160 := hash160(pubC)
	pkScript := standardP2PKHScript(h160[:])
	outVal := fund.Value - 500_000
	parent, err := signWalletP2PKHSpend(fund, priv, pubC, pkScript, outVal)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	midVal := outVal - 300_000
	child1, err := signWalletP2PKHSpendWithScriptSig(walletUTXOFromTx(parent, 0, outVal), priv, pubC, pkScript, midVal, make([]byte, 900))
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	child2 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: child1.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: midVal - 300_000, PkScript: pkScript}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, child2, 0)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	child2.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	pHex, err := txHex(parent)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	c1Hex, err := txHex(child1)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	c2Hex, err := txHex(child2)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{
		Template:         "package_descendant_size",
		WantAccept:       false,
		WantRejectReason: "too-long-mempool-chain",
		PrepTxHex:        []string{pHex, c1Hex},
		ProbeTxHex:       c2Hex,
	}, nil
}

func walletProbePQCommitmentOpReturn(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO) (StatefulLiveProbe, error) {
	h160 := hash160(pubC)
	pkScript := standardP2PKHScript(h160[:])
	commit := make([]byte, 32)
	pqScript, err := BuildPQCommitmentScript(PQTagDilithium, commit)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	changeVal := fund.Value - 1_000_000
	spend, err := signWalletP2PKHSpendMulti(fund, priv, pubC, 1, []wire.TxOut{
		{Value: changeVal, PkScript: pkScript},
		{Value: 0, PkScript: pqScript},
	})
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	probeHex, err := txHex(spend)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{Template: "pq_commitment_op_return", WantAccept: true, ProbeTxHex: probeHex}, nil
}

func walletProbePQCarrierP2SH(priv *secp256k1.PrivateKey, pubC []byte, fund WalletFundingUTXO) (StatefulLiveProbe, error) {
	h160 := hash160(pubC)
	pkScript := standardP2PKHScript(h160[:])
	carrier := BuildPQCarrierP2SHScriptPubKey()
	carrierVal := int64(1_000_000)
	changeVal := fund.Value - carrierVal - 500_000
	spend, err := signWalletP2PKHSpendMulti(fund, priv, pubC, 2, []wire.TxOut{
		{Value: changeVal, PkScript: pkScript},
		{Value: carrierVal, PkScript: carrier},
	})
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	probeHex, err := txHex(spend)
	if err != nil {
		return StatefulLiveProbe{}, err
	}
	return StatefulLiveProbe{Template: "pq_carrier_p2sh_accept", WantAccept: true, ProbeTxHex: probeHex}, nil
}

func signWalletP2PKHSpendMulti(fund WalletFundingUTXO, priv *secp256k1.PrivateKey, pubC []byte, version int32, outs []wire.TxOut) (*wire.Tx, error) {
	var totalOut int64
	for _, o := range outs {
		if o.Value > 0 && IsOutputDustEffective(o, DefaultStandardPolicy(), DefaultMinRelayTxFeePerKB) {
			return nil, fmt.Errorf("output below dust")
		}
		totalOut += o.Value
	}
	if totalOut >= fund.Value {
		return nil, fmt.Errorf("outputs exceed input")
	}
	tx := &wire.Tx{
		Version: version,
		Vin: []wire.TxIn{{
			PrevHash: fund.PrevHash,
			PrevIdx:  fund.PrevIdx,
			Sequence: 0xffffffff,
		}},
		Vout: outs,
	}
	digest, err := wire.CalcSignatureHashLegacy(fund.PkScript, wire.SigHashAll, tx, 0)
	if err != nil {
		return nil, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	tx.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	return tx, nil
}

func signWalletP2PKHSpend(fund WalletFundingUTXO, priv *secp256k1.PrivateKey, pubC []byte, outScript []byte, outVal int64) (*wire.Tx, error) {
	return signWalletP2PKHSpendWithScriptSig(fund, priv, pubC, outScript, outVal, nil)
}

func signWalletP2PKHSpendWithScriptSig(fund WalletFundingUTXO, priv *secp256k1.PrivateKey, pubC []byte, outScript []byte, outVal int64, extraScript []byte) (*wire.Tx, error) {
	if outVal <= HardDustLimitKoinu {
		return nil, fmt.Errorf("output below dust")
	}
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: fund.PrevHash,
			PrevIdx:  fund.PrevIdx,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: outVal, PkScript: outScript}},
	}
	digest, err := wire.CalcSignatureHashLegacy(fund.PkScript, wire.SigHashAll, tx, 0)
	if err != nil {
		return nil, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	scriptSig := buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)
	if len(extraScript) > 0 {
		scriptSig = append(scriptSig, extraScript...)
	}
	tx.Vin[0].Script = scriptSig
	return tx, nil
}

func walletUTXOFromTx(tx *wire.Tx, vout uint32, value int64) WalletFundingUTXO {
	return WalletFundingUTXO{
		PrevHash: tx.TxHash(),
		PrevIdx:  vout,
		Value:    value,
		PkScript: tx.Vout[vout].PkScript,
	}
}

func loadMempoolVector(template string) (MempoolDifferentialVector, error) {
	vecs, err := LoadMempoolDifferentialVectors()
	if err != nil {
		return MempoolDifferentialVector{}, err
	}
	for _, v := range vecs {
		if v.Template == template {
			return v, nil
		}
	}
	return MempoolDifferentialVector{}, fmt.Errorf("template %q not in corpus", template)
}

func txHex(tx *wire.Tx) (string, error) {
	raw, err := tx.Serialize()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
