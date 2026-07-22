// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/pqcrypto"
	"dogego/wire"
)

// PQCarrierBuildPlan holds unsigned TX_C and TX_R templates plus verifier metadata.
type PQCarrierBuildPlan struct {
	Scheme      pqcrypto.Scheme
	OPReturnTag string
	CarrierTag8 string
	PartTotal   int
	TXBase      *wire.Tx
	TXC         *wire.Tx
	TXR         *wire.Tx
	Commitment  [32]byte
	PK          []byte
	Sig         []byte
	Sighash32   [32]byte
}

// BuildPQCarrierTransactions appends OP_RETURN + P2SH carrier outputs to txBase and builds TX_R reveal.
func BuildPQCarrierTransactions(txBase *wire.Tx, scheme pqcrypto.Scheme, pk, sk []byte, inputIdx int, pkScript []byte, hashType uint32, carrierValue int64) (*PQCarrierBuildPlan, error) {
	if txBase == nil || scheme == nil {
		return nil, fmt.Errorf("pq carrier: missing tx or scheme")
	}
	if len(txBase.Vout) == 0 {
		return nil, fmt.Errorf("pq carrier: tx_base needs outputs")
	}
	if carrierValue < pqCarrierMinOutputKoinu {
		carrierValue = pqCarrierMinOutputKoinu
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, hashType, txBase, inputIdx)
	if err != nil {
		return nil, err
	}
	sig, err := scheme.Sign(sk, digest[:])
	if err != nil {
		return nil, err
	}
	full := append(append([]byte(nil), pk...), sig...)
	commit := scheme.Commit(pk, sig)
	commitScript, err := BuildPQCommitmentScript(scheme.OPReturnTag(), commit[:])
	if err != nil {
		return nil, err
	}
	algo, ok := PQCarrierAlgoForOPReturnTag(scheme.OPReturnTag())
	if !ok {
		return nil, fmt.Errorf("pq carrier: unknown op tag")
	}
	partTotal := scheme.PartTotal()
	carrierSPK := BuildPQCarrierP2SHScriptPubKey()
	totalCarrierCost := carrierValue * int64(partTotal)

	txc := cloneTx(txBase)
	if txc.Vout[0].Value < totalCarrierCost {
		return nil, fmt.Errorf("pq carrier: insufficient change for %d carrier outputs", partTotal)
	}
	txc.Vout[0].Value -= totalCarrierCost
	txc.Vout = append(txc.Vout, wire.TxOut{Value: 0, PkScript: commitScript})
	for i := 0; i < partTotal; i++ {
		txc.Vout = append(txc.Vout, wire.TxOut{Value: carrierValue, PkScript: append([]byte(nil), carrierSPK...)})
	}

	txr := &wire.Tx{Version: txc.Version, LockTime: txc.LockTime}
	carrierStart := len(txc.Vout) - partTotal
	for i := 0; i < partTotal; i++ {
		voutIdx := carrierStart + i
		prevHash := txc.TxHash()
		chunks, err := SplitPQCarrierPartPayload(full, i, partTotal)
		if err != nil {
			return nil, err
		}
		hdr, err := BuildPQCarrierHDR8(i, partTotal, len(pk), len(full))
		if err != nil {
			return nil, err
		}
		scriptSig, err := BuildPQCarrierPartScriptSig(algo.CarrierTag8, hdr, chunks)
		if err != nil {
			return nil, err
		}
		txr.Vin = append(txr.Vin, wire.TxIn{
			PrevHash: prevHash,
			PrevIdx:  uint32(voutIdx),
			Sequence: 0xffffffff,
			Script:   scriptSig,
		})
	}
	// Reveal fee paid from carrier inputs; optional change back to payer.
	change := carrierValue*int64(partTotal) - 1_000_000
	if change > 0 && len(txc.Vout) > 0 {
		txr.Vout = append(txr.Vout, wire.TxOut{Value: change, PkScript: append([]byte(nil), txc.Vout[0].PkScript...)})
	}

	var sigh [32]byte
	copy(sigh[:], digest[:])
	plan := &PQCarrierBuildPlan{
		Scheme:      scheme,
		OPReturnTag: scheme.OPReturnTag(),
		CarrierTag8: algo.CarrierTag8,
		PartTotal:   partTotal,
		TXBase:      cloneTx(txBase),
		TXC:         txc,
		TXR:         txr,
		Commitment:  commit,
		PK:          append([]byte(nil), pk...),
		Sig:         append([]byte(nil), sig...),
		Sighash32:   sigh,
	}
	RefreshPQCarrierTXRPrevouts(txc, txr)
	return plan, nil
}

// RefreshPQCarrierTXRPrevouts sets TX_R inputs to spend the carrier outputs of TX_C.
func RefreshPQCarrierTXRPrevouts(txc, txr *wire.Tx) {
	if txc == nil || txr == nil {
		return
	}
	partTotal := len(txr.Vin)
	if partTotal == 0 {
		return
	}
	carrierStart := len(txc.Vout) - partTotal
	prev := txc.TxHash()
	for i := range txr.Vin {
		txr.Vin[i].PrevHash = prev
		txr.Vin[i].PrevIdx = uint32(carrierStart + i)
	}
}

// VerifyPQCarrierPair performs verifier-side checks for TX_C + TX_R (no consensus enforcement).
func VerifyPQCarrierPair(txc, txr *wire.Tx, inputIdx int, pkScript []byte, hashType uint32, scheme pqcrypto.Scheme) (map[string]interface{}, error) {
	if txc == nil || txr == nil {
		return nil, fmt.Errorf("pq carrier: missing txc or txr")
	}
	commit, _, ok := DetectPQCommitmentInTx(txc)
	if !ok {
		return nil, fmt.Errorf("pq carrier: txc missing OP_RETURN commitment")
	}
	parts, err := ParsePQCarrierTXR(txr)
	if err != nil {
		return nil, err
	}
	if err := verifyPQCarrierTXRLinkage(txc, txr, parts); err != nil {
		return nil, err
	}
	mat, err := ReassemblePQCarrierParts(parts)
	if err != nil {
		return nil, err
	}
	if commit.Tag != mat.OPReturnTag {
		return nil, fmt.Errorf("pq carrier: commitment tag mismatch")
	}
	if !equalHex(commit.Commitment, mat.Commitment[:]) {
		return nil, fmt.Errorf("pq carrier: commitment mismatch")
	}
	base, _, err := ReconstructTXBaseFromTXC(txc)
	if err != nil {
		return nil, err
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, hashType, base, inputIdx)
	if err != nil {
		return nil, err
	}
	if scheme == nil {
		scheme, _ = pqcrypto.ByOPReturnTag(mat.OPReturnTag)
	}
	pqVerify := "skipped"
	if scheme != nil {
		if scheme.Verify(mat.PK, digest[:], mat.Sig) {
			pqVerify = "passed"
		} else {
			pqVerify = "failed"
		}
	}
	txcID := txc.TxHash()
	txrID := txr.TxHash()
	out := PQCarrierVerifyFields(mat, pqVerify, hexLower(txcID[:]), hexLower(txrID[:]))
	out["commitment"] = commit.Commitment
	out["sighash32"] = hexLower(digest[:])
	out["commitment_match"] = true
	out["linkage_ok"] = true
	out["pq_verify"] = pqVerify
	out["valid"] = pqVerify == "passed"
	out["verify_note"] = "Verifier-side PQ carrier check only; secp256k1/P2SH remains authoritative"
	if pqVerify == "skipped" {
		out["verify_note"] = out["verify_note"].(string) + "; PQ scheme unavailable for crypto verify"
	}
	return out, nil
}

// verifyPQCarrierTXRLinkage checks TX_R inputs spend the carrier P2SH outputs of TX_C.
func verifyPQCarrierTXRLinkage(txc, txr *wire.Tx, parts []*PQCarrierPart) error {
	partTotal := len(parts)
	if partTotal == 0 {
		return fmt.Errorf("pq carrier: no carrier parts")
	}
	if len(txr.Vin) != partTotal {
		return fmt.Errorf("pq carrier: txr vin count mismatch")
	}
	carrierCount, _ := CountPQCarrierOutputs(txc)
	if carrierCount != partTotal {
		return fmt.Errorf("pq carrier: txc carrier output count mismatch")
	}
	txcID := txc.TxHash()
	carrierStart := len(txc.Vout) - partTotal
	for i, in := range txr.Vin {
		if in.PrevHash != txcID {
			return fmt.Errorf("pq carrier: txr input %d prev txid mismatch", i)
		}
		wantIdx := carrierStart + i
		if int(in.PrevIdx) != wantIdx {
			return fmt.Errorf("pq carrier: txr input %d prev index mismatch", i)
		}
		if wantIdx >= len(txc.Vout) || !IsPQCarrierScriptPubKey(txc.Vout[wantIdx].PkScript) {
			return fmt.Errorf("pq carrier: txc output %d not carrier p2sh", wantIdx)
		}
	}
	return nil
}

func cloneTx(tx *wire.Tx) *wire.Tx {
	if tx == nil {
		return nil
	}
	out := &wire.Tx{Version: tx.Version, LockTime: tx.LockTime}
	out.Vin = append([]wire.TxIn(nil), tx.Vin...)
	out.Vout = make([]wire.TxOut, len(tx.Vout))
	for i, v := range tx.Vout {
		out.Vout[i] = wire.TxOut{Value: v.Value, PkScript: append([]byte(nil), v.PkScript...)}
	}
	return out
}

func hexLower(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}

func equalHex(a string, b32 []byte) bool {
	return stringsEqualFold(a, hexLower(b32))
}

func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
