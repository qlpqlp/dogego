// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/consensus"
	"dogego/extensions"
)

// GenerateProofParams configures generateproof RPC.
type GenerateProofParams struct {
	Payload          string `json:"payload"`
	PayloadEncoding  string `json:"payload_encoding"` // text, base64, hex
	ProofKind        string `json:"proof_kind"`       // commitment, groth16
	Groth16ProofHex  string `json:"groth16_proof_hex,omitempty"`
	DemoGroth16      bool   `json:"demo_groth16,omitempty"`
	TransactionID    string `json:"transaction_id,omitempty"`
	BlockHash        string `json:"block_hash,omitempty"`
	BlockHeight      int64  `json:"block_height,omitempty"`
	TransactionIndex uint32 `json:"transaction_index,omitempty"`
	Submit           bool   `json:"submit,omitempty"`
	Metadata         string `json:"metadata,omitempty"`
}

func decodeGenerateProofParams(params []json.RawMessage) (GenerateProofParams, error) {
	if len(params) < 1 {
		return GenerateProofParams{}, fmt.Errorf("want generate params object")
	}
	var req GenerateProofParams
	if err := json.Unmarshal(params[0], &req); err != nil {
		return GenerateProofParams{}, err
	}
	return req, nil
}

func (e *Extension) rpcGenerateProof(host extensions.Host, req GenerateProofParams) (map[string]interface{}, error) {
	payload, payloadKind, err := decodePayloadBytes(req.Payload, req.PayloadEncoding)
	if err != nil {
		return nil, err
	}
	kind := strings.ToLower(strings.TrimSpace(req.ProofKind))
	if kind == "" {
		kind = "commitment"
	}

	var p Proof
	var proofKindLabel string
	switch kind {
	case "commitment":
		p, err = buildCommitmentProof(payload, payloadKind, req.Metadata)
		proofKindLabel = "commitment"
	case "groth16":
		p, err = buildGroth16ProofFromPayload(payload, payloadKind, req.Groth16ProofHex, req.DemoGroth16, req.Metadata)
		proofKindLabel = "groth16"
	default:
		return nil, fmt.Errorf("unsupported proof_kind %q (use commitment or groth16)", kind)
	}
	if err != nil {
		return nil, err
	}

	if txid := strings.TrimSpace(req.TransactionID); txid != "" {
		p.TransactionID = txid
		p.BlockHash = strings.TrimSpace(req.BlockHash)
		p.BlockHeight = req.BlockHeight
		p.TransactionIndex = req.TransactionIndex
		p, err = NormalizeProof(p)
		if err != nil {
			return nil, err
		}
	}

	payloadHash := sha256.Sum256(payload)
	anchorScript, _ := consensus.BuildZKAnchorScript(mustDecode32(p.ProofHash))
	out := map[string]interface{}{
		"proof":            p,
		"proof_kind":       proofKindLabel,
		"payload_sha256":   hex.EncodeToString(payloadHash[:]),
		"payload_size":     len(payload),
		"zkdg_script_hex":  hex.EncodeToString(anchorScript),
		"workflow": []string{
			"1. Broadcast a Dogecoin transaction carrying your payload or a reference (witness / OP_RETURN).",
			"2. Wait for confirmation, then set transaction_id, block_hash, block_height on the proof.",
			"3. Optionally attach ZKDG OP_RETURN using zkdg_script_hex (secondary anchor per #3869 discussion).",
			"4. Call submitproof or set submit=true when the anchor tx is confirmed.",
		},
	}

	if req.Submit {
		if strings.TrimSpace(p.TransactionID) == "" {
			return nil, fmt.Errorf("submit requires transaction_id, block_hash, block_height")
		}
		sub, err := e.rpcSubmitProof(host, p)
		if err != nil {
			return nil, err
		}
		out["submit"] = sub
	}
	return out, nil
}

func buildCommitmentProof(payload []byte, payloadKind, extraMeta string) (Proof, error) {
	hash := sha256.Sum256(payload)
	tag := sha256.Sum256(commitmentDomain)
	wire := buildZKCMWire(payloadKind, len(payload), hash[:], tag[:])
	meta := map[string]interface{}{
		"payload_kind":   payloadKind,
		"payload_size":   len(payload),
		"payload_sha256": hex.EncodeToString(hash[:]),
		"proof_kind":     "commitment",
		"note":           "SHA256 commitment proof (overlay). Not a Groth16 ZK-SNARK; binds payload hash to a confirmed tx.",
	}
	if strings.TrimSpace(extraMeta) != "" {
		meta["user_metadata"] = extraMeta
	}
	raw, _ := json.Marshal(meta)
	p := Proof{
		ProofData:    hex.EncodeToString(wire),
		ProofType:    ProofModeCommitment,
		PublicInputs: []string{hex.EncodeToString(hash[:]), hex.EncodeToString(tag[:])},
		Metadata:     string(raw),
	}
	return NormalizeProof(p)
}

func buildGroth16ProofFromPayload(payload []byte, payloadKind, externalHex string, demo bool, extraMeta string) (Proof, error) {
	hash := sha256.Sum256(payload)
	bind := sha256.Sum256(append([]byte("dogego.zkl2.zkpg.v1"), payload...))

	var proofBytes []byte
	var publicInputs []string
	meta := map[string]interface{}{
		"payload_kind":   payloadKind,
		"payload_size":   len(payload),
		"payload_sha256": hex.EncodeToString(hash[:]),
		"proof_kind":     "groth16",
	}

	externalHex = strings.TrimSpace(externalHex)
	switch {
	case externalHex != "":
		b, err := hex.DecodeString(externalHex)
		if err != nil {
			return Proof{}, fmt.Errorf("groth16_proof_hex: %w", err)
		}
		proofBytes = b
		publicInputs = []string{hex.EncodeToString(hash[:]), hex.EncodeToString(bind[:])}
		meta["note"] = "Groth16 proof supplied externally; verify with loaded VK or inline verifying_key."
	case demo:
		_, proof, demoInput := groth16DemoVector()
		wire := buildZKPGWire(proof, [][]byte{demoInput})
		proofBytes = wire
		publicInputs = []string{hex.EncodeToString(demoInput)}
		meta["demo"] = true
		meta["note"] = "Demo Groth16 vector only does NOT prove your payload hash. Use groth16_proof_hex from an external prover for real ZK."
	default:
		return Proof{}, fmt.Errorf("groth16 requires groth16_proof_hex from your prover, or demo_groth16=true for a pairing smoke test")
	}

	if strings.TrimSpace(extraMeta) != "" {
		meta["user_metadata"] = extraMeta
	}
	raw, _ := json.Marshal(meta)
	p := Proof{
		ProofData:    hex.EncodeToString(proofBytes),
		ProofType:    ProofModeGroth16,
		PublicInputs: publicInputs,
		Metadata:     string(raw),
	}
	return NormalizeProof(p)
}

func buildZKPGWire(proof []byte, pis [][]byte) []byte {
	var wireBlob []byte
	wireBlob = append(wireBlob, []byte(groth16WireMagic)...)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(proof)))
	wireBlob = append(wireBlob, n[:]...)
	binary.LittleEndian.PutUint32(n[:], uint32(len(pis)))
	wireBlob = append(wireBlob, n[:]...)
	wireBlob = append(wireBlob, proof...)
	for _, p := range pis {
		wireBlob = append(wireBlob, p...)
	}
	return wireBlob
}