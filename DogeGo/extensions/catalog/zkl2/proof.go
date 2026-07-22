// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ProtocolID is the P2P overlay protocol name (peer negotiation).
const ProtocolID = "zkproof-v1"

const (
	MaxProofDataBytes = 256 * 1024
	ProofVersionV1    = 1
)

// Proof is a zero-knowledge proof anchored to a confirmed Dogecoin transaction.
// OP_CHECKZKP-style verification runs in the extension only (never on L1).
type Proof struct {
	ProofHash        string `json:"proof_hash"`
	TransactionID    string `json:"transaction_id"`
	BlockHash        string `json:"block_hash"`
	BlockHeight      int64  `json:"block_height"`
	TransactionIndex uint32 `json:"transaction_index"`
	ProofData        string `json:"proof_data"` // hex
	ProofVersion     uint32 `json:"proof_version"`
	CreatedTimestamp int64  `json:"created_timestamp"`
	ProofType        uint32 `json:"proof_type"` // 1 = Groth16; 2 = SHA256 commitment (overlay)
	PublicInputs     []string `json:"public_inputs,omitempty"`
	// VerifyingKey is optional flat snarkjs .vk hex (verify-time only; not hashed into proof_hash).
	VerifyingKey string `json:"verifying_key,omitempty"`
	// VerifyingKeyChunks is optional #3869 mode-0 stack layout: 6 × 80-byte hex chunks (verify-time only).
	VerifyingKeyChunks []string `json:"verifying_key_chunks,omitempty"`
	Metadata         string `json:"metadata,omitempty"`
	Signature        string `json:"signature,omitempty"`
}

// ProofCommitment is SHA256(BlockHash || TransactionID || ProofHash) - immutable anchor.
func ProofCommitment(blockHash, txid, proofHash string) (string, error) {
	bh, err := decodeHash32(blockHash)
	if err != nil {
		return "", err
	}
	ph, err := decodeHash32(proofHash)
	if err != nil {
		return "", err
	}
	txb, err := hex.DecodeString(strings.TrimSpace(txid))
	if err != nil {
		return "", fmt.Errorf("invalid txid hex")
	}
	h := sha256.New()
	h.Write(bh[:])
	h.Write(txb)
	h.Write(ph[:])
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeProofHash returns deterministic SHA256(canonical proof fields excluding proof_hash).
func ComputeProofHash(p Proof) (string, error) {
	canon := struct {
		TransactionID    string   `json:"transaction_id"`
		BlockHash        string   `json:"block_hash"`
		BlockHeight      int64    `json:"block_height"`
		TransactionIndex uint32   `json:"transaction_index"`
		ProofData        string   `json:"proof_data"`
		ProofVersion     uint32   `json:"proof_version"`
		ProofType        uint32   `json:"proof_type"`
		PublicInputs     []string `json:"public_inputs"`
	}{
		TransactionID:    strings.ToLower(strings.TrimSpace(p.TransactionID)),
		BlockHash:        strings.ToLower(strings.TrimSpace(p.BlockHash)),
		BlockHeight:      p.BlockHeight,
		TransactionIndex: p.TransactionIndex,
		ProofData:        strings.ToLower(strings.TrimSpace(p.ProofData)),
		ProofVersion:     p.ProofVersion,
		ProofType:        p.ProofType,
		PublicInputs:     append([]string(nil), p.PublicInputs...),
	}
	raw, err := json.Marshal(canon)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// NormalizeProof fills defaults and proof_hash.
func NormalizeProof(p Proof) (Proof, error) {
	if p.ProofVersion == 0 {
		p.ProofVersion = ProofVersionV1
	}
	if p.ProofType == 0 {
		p.ProofType = ProofModeGroth16
	}
	if p.CreatedTimestamp == 0 {
		p.CreatedTimestamp = time.Now().Unix()
	}
	if p.ProofHash == "" {
		h, err := ComputeProofHash(p)
		if err != nil {
			return p, err
		}
		p.ProofHash = h
	}
	return p, nil
}

// ValidateProofPayload checks proof fields and hash consistency (no chain anchor required).
func ValidateProofPayload(p Proof) error {
	p, err := NormalizeProof(p)
	if err != nil {
		return err
	}
	data, err := hex.DecodeString(strings.TrimSpace(p.ProofData))
	if err != nil {
		return fmt.Errorf("proof_data hex: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("proof_data empty")
	}
	if len(data) > MaxProofDataBytes {
		return fmt.Errorf("proof_data exceeds max %d bytes", MaxProofDataBytes)
	}
	want, err := ComputeProofHash(p)
	if err != nil {
		return err
	}
	if !strings.EqualFold(want, p.ProofHash) {
		return fmt.Errorf("proof_hash mismatch")
	}
	return nil
}

// ValidateProofStructure checks payload plus tx anchor fields for storage.
func ValidateProofStructure(p Proof) error {
	if err := ValidateProofPayload(p); err != nil {
		return err
	}
	if _, err := decodeHash32(p.BlockHash); err != nil {
		return fmt.Errorf("block_hash: %w", err)
	}
	if strings.TrimSpace(p.TransactionID) == "" {
		return fmt.Errorf("transaction_id required")
	}
	if p.BlockHeight < 0 {
		return fmt.Errorf("invalid block_height")
	}
	return nil
}

// ComputeProofRoot builds a Merkle root over proof hashes for one Dogecoin block (sorted by txid).
func ComputeProofRoot(proofs []Proof) (string, error) {
	if len(proofs) == 0 {
		return strings.Repeat("0", 64), nil
	}
	hashes := make([][32]byte, 0, len(proofs))
	type row struct {
		txid string
		h    [32]byte
	}
	var rows []row
	seen := make(map[string]struct{})
	for _, p := range proofs {
		if err := ValidateProofStructure(p); err != nil {
			return "", err
		}
		key := strings.ToLower(p.ProofHash)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		h, err := decodeHash32(p.ProofHash)
		if err != nil {
			return "", err
		}
		rows = append(rows, row{txid: strings.ToLower(p.TransactionID), h: h})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].txid < rows[j].txid })
	for _, r := range rows {
		hashes = append(hashes, r.h)
	}
	return merkleRootHex(hashes), nil
}

func merkleRootHex(leaves [][32]byte) string {
	if len(leaves) == 0 {
		return strings.Repeat("0", 64)
	}
	level := append([][32]byte(nil), leaves...)
	for len(level) > 1 {
		var next [][32]byte
		for i := 0; i < len(level); i += 2 {
			if i+1 == len(level) {
				next = append(next, level[i])
				continue
			}
			h := sha256.New()
			h.Write(level[i][:])
			h.Write(level[i+1][:])
			var n [32]byte
			copy(n[:], h.Sum(nil))
			next = append(next, n)
		}
		level = next
	}
	return hex.EncodeToString(level[0][:])
}
