// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"dogego/wire"
)

// Phase-1 P2SH carrier tags (8-byte scriptSig prefix).
const (
	PQCarrierTagFalconFull    = "FLC1FULL"
	PQCarrierTagDilithiumFull = "DIL2FULL"
	PQCarrierTagRaccoonFull   = "RCG4FULL"
)

const (
	pqCarrierHDRVersion     = 0x01
	pqCarrierChunksPerPart  = 3
	pqCarrierMaxChunkSize   = maxScriptElementSize // 520
	pqCarrierMaxPartPayload = pqCarrierChunksPerPart * pqCarrierMaxChunkSize
	pqCarrierRedeemLen      = 6
	pqCarrierMinOutputKoinu = 100_000_000 // 1 DOGE per BIP carrier output guidance
)

// PQCarrierMinOutputKoinu returns the recommended minimum carrier P2SH output value.
func PQCarrierMinOutputKoinu() int64 { return pqCarrierMinOutputKoinu }

// PQCarrierAlgo describes one PQ carrier algorithm profile.
type PQCarrierAlgo struct {
	CarrierTag8 string
	OPReturnTag string
	Scheme      string
	PartTotal   int
	ExpectedPK  int // 0 = use actual pk length from HDR8
}

// PQCarrierAlgos lists BIP Phase-1 carrier profiles.
var PQCarrierAlgos = map[string]PQCarrierAlgo{
	PQCarrierTagFalconFull: {
		CarrierTag8: PQCarrierTagFalconFull,
		OPReturnTag: PQTagFalcon,
		Scheme:      "falcon-512",
		PartTotal:   2,
		ExpectedPK:  897,
	},
	PQCarrierTagDilithiumFull: {
		CarrierTag8: PQCarrierTagDilithiumFull,
		OPReturnTag: PQTagDilithium,
		Scheme:      "dilithium2",
		PartTotal:   3,
		ExpectedPK:  1312,
	},
	PQCarrierTagRaccoonFull: {
		CarrierTag8: PQCarrierTagRaccoonFull,
		OPReturnTag: PQTagRaccoon,
		Scheme:      "raccoon-g-44",
		PartTotal:   24,
		ExpectedPK:  16144,
	},
}

// PQCarrierPart is one decoded TX_R carrier scriptSig part.
type PQCarrierPart struct {
	CarrierTag8  string
	Version      byte
	PartIndex    byte
	PartTotal    byte
	PKLen        uint16
	FullLen      uint16
	Chunks       [pqCarrierChunksPerPart][]byte
	Payload      []byte
	RedeemScript []byte
}

// PQCarrierMaterial is reassembled pk||sig from one or more carrier parts.
type PQCarrierMaterial struct {
	Scheme      string
	OPReturnTag string
	CarrierTag8 string
	PK          []byte
	Sig         []byte
	Commitment  [32]byte
}

// BuildPQCarrierRedeemScript returns canonical 5×OP_DROP + OP_TRUE redeem script.
func BuildPQCarrierRedeemScript() []byte {
	return []byte{0x75, 0x75, 0x75, 0x75, 0x75, 0x51}
}

// IsPQCarrierRedeemScript reports the fixed Phase-1 PQ carrier redeem template.
func IsPQCarrierRedeemScript(script []byte) bool {
	want := BuildPQCarrierRedeemScript()
	return len(script) == len(want) && string(script) == string(want)
}

// PQCarrierRedeemHash160 is HASH160 of the canonical carrier redeem script.
func PQCarrierRedeemHash160() [20]byte {
	return hash160(BuildPQCarrierRedeemScript())
}

// BuildPQCarrierP2SHScriptPubKey builds OP_HASH160 <h160> OP_EQUAL for the canonical carrier redeem.
func BuildPQCarrierP2SHScriptPubKey() []byte {
	h := PQCarrierRedeemHash160()
	return BuildP2SHScriptPubKey(h)
}

// BuildP2SHScriptPubKey builds standard P2SH scriptPubKey from HASH160(redeemScript).
func BuildP2SHScriptPubKey(h160 [20]byte) []byte {
	out := make([]byte, 23)
	out[0] = 0xa9
	out[1] = 0x14
	copy(out[2:22], h160[:])
	out[22] = 0x87
	return out
}

// IsPQCarrierScriptPubKey reports whether scriptPubKey is the canonical PQ carrier P2SH output.
func IsPQCarrierScriptPubKey(script []byte) bool {
	if !isP2SHScript(script) {
		return false
	}
	var want [20]byte
	copy(want[:], script[2:22])
	return want == PQCarrierRedeemHash160()
}

// PQCarrierAlgoForOPReturnTag maps FLC1/DIL2/RCG4 to carrier metadata.
func PQCarrierAlgoForOPReturnTag(tag string) (PQCarrierAlgo, bool) {
	for _, a := range PQCarrierAlgos {
		if a.OPReturnTag == tag {
			return a, true
		}
	}
	return PQCarrierAlgo{}, false
}

// PQCarrierAlgoForCarrierTag maps FLC1FULL/DIL2FULL/RCG4FULL to carrier metadata.
func PQCarrierAlgoForCarrierTag(tag8 string) (PQCarrierAlgo, bool) {
	a, ok := PQCarrierAlgos[tag8]
	return a, ok
}

// PQCommitmentFromMaterial computes SHA256(pk || sig).
func PQCommitmentFromMaterial(pk, sig []byte) [32]byte {
	h := sha256.New()
	_, _ = h.Write(pk)
	_, _ = h.Write(sig)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// BuildPQCarrierHDR8 encodes the 8-byte carrier part header.
func BuildPQCarrierHDR8(partIndex, partTotal int, pkLen, fullLen int) ([8]byte, error) {
	if partIndex < 0 || partIndex > 255 || partTotal < 1 || partTotal > 255 {
		return [8]byte{}, fmt.Errorf("pq carrier: invalid part index/total")
	}
	if pkLen < 0 || pkLen > 0xffff || fullLen < 0 || fullLen > 0xffff {
		return [8]byte{}, fmt.Errorf("pq carrier: invalid pk_len/full_len")
	}
	var hdr [8]byte
	hdr[0] = pqCarrierHDRVersion
	hdr[1] = byte(partIndex)
	hdr[2] = byte(partTotal)
	hdr[3] = 0x00
	binary.BigEndian.PutUint16(hdr[4:6], uint16(pkLen))
	binary.BigEndian.PutUint16(hdr[6:8], uint16(fullLen))
	return hdr, nil
}

// ParsePQCarrierHDR8 decodes the 8-byte carrier part header.
func ParsePQCarrierHDR8(hdr []byte) (partIndex, partTotal int, pkLen, fullLen int, err error) {
	if len(hdr) != 8 {
		return 0, 0, 0, 0, fmt.Errorf("pq carrier: hdr8 must be 8 bytes")
	}
	if hdr[0] != pqCarrierHDRVersion {
		return 0, 0, 0, 0, fmt.Errorf("pq carrier: unsupported hdr version %d", hdr[0])
	}
	if hdr[3] != 0x00 {
		return 0, 0, 0, 0, fmt.Errorf("pq carrier: reserved hdr byte must be zero")
	}
	partIndex = int(hdr[1])
	partTotal = int(hdr[2])
	pkLen = int(binary.BigEndian.Uint16(hdr[4:6]))
	fullLen = int(binary.BigEndian.Uint16(hdr[6:8]))
	if partTotal < 1 || partIndex >= partTotal {
		return 0, 0, 0, 0, fmt.Errorf("pq carrier: invalid part index %d of %d", partIndex, partTotal)
	}
	if pkLen < 0 || fullLen < pkLen {
		return 0, 0, 0, 0, fmt.Errorf("pq carrier: invalid pk_len/full_len")
	}
	return partIndex, partTotal, pkLen, fullLen, nil
}

// SplitPQCarrierPartPayload splits FULL into one part's up-to-three <=520-byte chunks.
func SplitPQCarrierPartPayload(full []byte, partIndex, partTotal int) (chunks [pqCarrierChunksPerPart][]byte, err error) {
	if partIndex < 0 || partTotal < 1 || partIndex >= partTotal {
		return chunks, fmt.Errorf("pq carrier: invalid part index")
	}
	start := partIndex * pqCarrierMaxPartPayload
	if start > len(full) {
		return chunks, nil
	}
	remain := full[start:]
	for i := 0; i < pqCarrierChunksPerPart; i++ {
		if len(remain) == 0 {
			break
		}
		n := len(remain)
		if n > pqCarrierMaxChunkSize {
			n = pqCarrierMaxChunkSize
		}
		chunks[i] = append([]byte(nil), remain[:n]...)
		remain = remain[n:]
	}
	return chunks, nil
}

// BuildPQCarrierPartScriptSig encodes one TX_R carrier vin scriptSig push sequence.
func BuildPQCarrierPartScriptSig(carrierTag8 string, hdr [8]byte, chunks [pqCarrierChunksPerPart][]byte) ([]byte, error) {
	if len(carrierTag8) != 8 {
		return nil, fmt.Errorf("pq carrier: tag8 must be 8 bytes")
	}
	if _, ok := PQCarrierAlgos[carrierTag8]; !ok {
		return nil, fmt.Errorf("pq carrier: unknown tag8 %q", carrierTag8)
	}
	var parts [][]byte
	parts = append(parts, []byte(carrierTag8), hdr[:])
	for i := 0; i < pqCarrierChunksPerPart; i++ {
		parts = append(parts, chunks[i])
	}
	parts = append(parts, BuildPQCarrierRedeemScript())
	var out []byte
	for _, p := range parts {
		out = append(out, buildCarrierPushScript(p)...)
	}
	return out, nil
}

// buildCarrierPushScript encodes a push up to maxScriptElementSize (520).
func buildCarrierPushScript(data []byte) []byte {
	if len(data) <= 75 {
		return buildSinglePushScript(data)
	}
	if len(data) <= 255 {
		return buildSinglePushScript(data)
	}
	if len(data) <= maxScriptElementSize {
		b := make([]byte, 0, 3+len(data))
		n := len(data)
		b = append(b, 0x4d, byte(n), byte(n>>8))
		return append(b, data...)
	}
	return buildSinglePushScript(data[:maxScriptElementSize])
}

// ParsePQCarrierPartScriptSig parses one carrier reveal scriptSig.
func ParsePQCarrierPartScriptSig(scriptSig []byte) (*PQCarrierPart, error) {
	if !isPushOnly(scriptSig) {
		return nil, fmt.Errorf("pq carrier: scriptSig must be push-only")
	}
	pushes, err := ScriptSigPushes(scriptSig)
	if err != nil {
		return nil, err
	}
	need := 2 + pqCarrierChunksPerPart + 1
	if len(pushes) != need {
		return nil, fmt.Errorf("pq carrier: want %d pushes, got %d", need, len(pushes))
	}
	tag := string(pushes[0])
	if len(tag) != 8 {
		return nil, fmt.Errorf("pq carrier: tag8 must be 8 bytes")
	}
	if _, ok := PQCarrierAlgos[tag]; !ok {
		return nil, fmt.Errorf("pq carrier: unknown tag8 %q", tag)
	}
	hdr := pushes[1]
	if len(hdr) != 8 {
		return nil, fmt.Errorf("pq carrier: hdr8 must be 8 bytes")
	}
	partIndex, partTotal, pkLen, fullLen, err := ParsePQCarrierHDR8(hdr)
	if err != nil {
		return nil, err
	}
	redeem := pushes[len(pushes)-1]
	if !IsPQCarrierRedeemScript(redeem) {
		return nil, fmt.Errorf("pq carrier: invalid redeem script")
	}
	var part PQCarrierPart
	part.CarrierTag8 = tag
	part.Version = hdr[0]
	part.PartIndex = byte(partIndex)
	part.PartTotal = byte(partTotal)
	part.PKLen = uint16(pkLen)
	part.FullLen = uint16(fullLen)
	part.RedeemScript = append([]byte(nil), redeem...)
	for i := 0; i < pqCarrierChunksPerPart; i++ {
		part.Chunks[i] = append([]byte(nil), pushes[2+i]...)
		part.Payload = append(part.Payload, part.Chunks[i]...)
	}
	if len(part.Payload) > pqCarrierMaxPartPayload {
		return nil, fmt.Errorf("pq carrier: part payload exceeds max")
	}
	return &part, nil
}

// ReassemblePQCarrierParts concatenates ordered parts and splits pk||sig.
func ReassemblePQCarrierParts(parts []*PQCarrierPart) (*PQCarrierMaterial, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("pq carrier: no parts")
	}
	algo, ok := PQCarrierAlgos[parts[0].CarrierTag8]
	if !ok {
		return nil, fmt.Errorf("pq carrier: unknown tag8")
	}
	partTotal := int(parts[0].PartTotal)
	pkLen := int(parts[0].PKLen)
	fullLen := int(parts[0].FullLen)
	if partTotal < 1 || fullLen < pkLen {
		return nil, fmt.Errorf("pq carrier: invalid metadata")
	}
	byIdx := make([][]byte, partTotal)
	for _, p := range parts {
		if p.CarrierTag8 != parts[0].CarrierTag8 {
			return nil, fmt.Errorf("pq carrier: mixed carrier tags")
		}
		if int(p.PartTotal) != partTotal || int(p.PKLen) != pkLen || int(p.FullLen) != fullLen {
			return nil, fmt.Errorf("pq carrier: inconsistent part metadata")
		}
		idx := int(p.PartIndex)
		if idx < 0 || idx >= partTotal || byIdx[idx] != nil {
			return nil, fmt.Errorf("pq carrier: duplicate or invalid part index %d", idx)
		}
		byIdx[idx] = append([]byte(nil), p.Payload...)
	}
	var full []byte
	for i := 0; i < partTotal; i++ {
		if byIdx[i] == nil {
			return nil, fmt.Errorf("pq carrier: missing part %d", i)
		}
		full = append(full, byIdx[i]...)
	}
	if len(full) < fullLen {
		return nil, fmt.Errorf("pq carrier: assembled payload too short")
	}
	full = full[:fullLen]
	pk := append([]byte(nil), full[:pkLen]...)
	sig := append([]byte(nil), full[pkLen:]...)
	commit := PQCommitmentFromMaterial(pk, sig)
	return &PQCarrierMaterial{
		Scheme:      algo.Scheme,
		OPReturnTag: algo.OPReturnTag,
		CarrierTag8: algo.CarrierTag8,
		PK:          pk,
		Sig:         sig,
		Commitment:  commit,
	}, nil
}

// ParsePQCarrierTXR extracts all carrier parts from a TX_R transaction.
func ParsePQCarrierTXR(tx *wire.Tx) ([]*PQCarrierPart, error) {
	var parts []*PQCarrierPart
	for i := range tx.Vin {
		part, err := ParsePQCarrierPartScriptSig(tx.Vin[i].Script)
		if err != nil {
			continue
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("pq carrier: no carrier scriptSig inputs")
	}
	return parts, nil
}

// DetectPQCommitmentInTx returns the first canonical OP_RETURN PQ commitment in tx.
func DetectPQCommitmentInTx(tx *wire.Tx) (PQCommitment, int, bool) {
	for i, out := range tx.Vout {
		if c, ok := DetectPQCommitment(out.PkScript); ok {
			return c, i, true
		}
	}
	return PQCommitment{}, -1, false
}

// CountPQCarrierOutputs counts P2SH carrier outputs and their total value.
func CountPQCarrierOutputs(tx *wire.Tx) (count int, totalValue int64) {
	for _, out := range tx.Vout {
		if IsPQCarrierScriptPubKey(out.PkScript) {
			count++
			totalValue += out.Value
		}
	}
	return count, totalValue
}

// ReconstructTXBaseFromTXC removes OP_RETURN PQ commitment and carrier P2SH outputs,
// restoring carrier value to vout[0] per BIP TX_BASE reconstruction.
func ReconstructTXBaseFromTXC(txc *wire.Tx) (*wire.Tx, int64, error) {
	if txc == nil || len(txc.Vout) == 0 {
		return nil, 0, fmt.Errorf("pq carrier: txc missing outputs")
	}
	carrierCount, carrierValue := CountPQCarrierOutputs(txc)
	base := &wire.Tx{
		Version:  txc.Version,
		LockTime: txc.LockTime,
		Vin:      append([]wire.TxIn(nil), txc.Vin...),
	}
	var restored int64
	for _, out := range txc.Vout {
		if _, ok := DetectPQCommitment(out.PkScript); ok {
			continue
		}
		if IsPQCarrierScriptPubKey(out.PkScript) {
			continue
		}
		base.Vout = append(base.Vout, out)
	}
	if len(base.Vout) == 0 {
		return nil, 0, fmt.Errorf("pq carrier: tx_base would have no outputs")
	}
	base.Vout[0].Value += carrierValue
	restored = carrierValue
	if carrierCount > 0 && restored == 0 {
		return nil, 0, fmt.Errorf("pq carrier: carrier outputs had zero value")
	}
	return base, restored, nil
}

// PQCarrierFields returns RPC/explorer metadata for a carrier part or commitment.
func PQCarrierFields(part *PQCarrierPart) map[string]interface{} {
	if part == nil {
		return nil
	}
	algo, _ := PQCarrierAlgos[part.CarrierTag8]
	return map[string]interface{}{
		"dogego_pqc_mode":         "carrier_scriptsig",
		"dogego_pqc_carrier_tag8": part.CarrierTag8,
		"dogego_pqc_scheme":       algo.Scheme,
		"dogego_pqc_tag":          algo.OPReturnTag,
		"dogego_pqc_part_index":   int(part.PartIndex),
		"dogego_pqc_part_total":   int(part.PartTotal),
		"dogego_pqc_pk_len":       int(part.PKLen),
		"dogego_pqc_full_len":     int(part.FullLen),
	}
}

// PQCarrierVerifyFields builds verifier-side metadata for a matched TX_C/TX_R pair.
func PQCarrierVerifyFields(mat *PQCarrierMaterial, pqVerify string, matchedTXC string, txrTXID string) map[string]interface{} {
	if mat == nil {
		return nil
	}
	out := map[string]interface{}{
		"dogego_pqc_mode":         "carrier_scriptsig",
		"dogego_pqc_source":       "carrier_scriptsig",
		"dogego_pqc_scheme":       mat.Scheme,
		"dogego_pqc_tag":          mat.OPReturnTag,
		"dogego_pqc_carrier_tag8": mat.CarrierTag8,
		"dogego_pqc_commitment":   hex.EncodeToString(mat.Commitment[:]),
		"dogego_pqc_pk_len":       len(mat.PK),
		"dogego_pqc_sig_len":      len(mat.Sig),
		"dogego_pqc_pq_verify":    pqVerify,
		"dogego_pqc_note":         "Verifier-side PQ evidence only; not consensus-enforced",
	}
	if matchedTXC != "" {
		out["dogego_pqc_matched_txc_txid"] = strings.ToLower(matchedTXC)
	}
	if txrTXID != "" {
		out["dogego_pqc_txr_txid"] = strings.ToLower(txrTXID)
	}
	if len(mat.PK) >= 16 {
		out["dogego_pqc_pk_prefix"] = hex.EncodeToString(mat.PK[:16])
	}
	if len(mat.Sig) >= 16 {
		out["dogego_pqc_sig_prefix"] = hex.EncodeToString(mat.Sig[:16])
	}
	return out
}
