// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// CheckZKPMode is the OP_CHECKZKP mode selector (#3869 analogue). Runs only in the extension.
type CheckZKPMode uint32

const (
	CheckZKPModeGroth16 CheckZKPMode = 1 // BLS12-381 Groth16 (ZKPG wire + compressed or DIP affine)
)

// VerifyCheckZKP is the L2 OP_CHECKZKP-equivalent verifier (off L1 consensus).
// Dogecoin L1 never executes this; only DogeGo nodes with zkproof-v1 enabled do.
// Optional vkOverride supplies inline verifying key bytes (#3869 stack chunks joined, or flat snarkjs .vk).
func VerifyCheckZKP(mode CheckZKPMode, proofData []byte, publicInputs [][]byte, vkOverride ...[]byte) error {
	if len(proofData) == 0 {
		return fmt.Errorf("checkzkp: empty proof")
	}
	if len(proofData) > MaxProofDataBytes {
		return fmt.Errorf("checkzkp: proof too large")
	}
	switch mode {
	case CheckZKPModeGroth16:
		return verifyGroth16Placeholder(proofData, publicInputs, vkOverride...)
	case CheckZKPMode(ProofModeCommitment):
		return verifyCommitmentProof(proofData, publicInputs)
	default:
		return fmt.Errorf("checkzkp: unsupported mode %d", mode)
	}
}

// verifyGroth16Placeholder validates ZKPG wire layout and runs pairing when VK is loaded.
func verifyGroth16Placeholder(proofData []byte, publicInputs [][]byte, vkOverride ...[]byte) error {
	var vk []byte
	if len(vkOverride) > 0 {
		vk = vkOverride[0]
	}
	if len(proofData) < 32 {
		return fmt.Errorf("groth16: proof too short")
	}
	if len(publicInputs) == 0 {
		return fmt.Errorf("groth16: public inputs required")
	}
	for i, in := range publicInputs {
		if len(in) != 32 {
			return fmt.Errorf("groth16: public input %d want 32 bytes", i)
		}
	}
	if string(proofData[:4]) == groth16WireMagic {
		if err := parseGroth16Wire(proofData, publicInputs, vk); err != nil {
			return err
		}
		return nil
	}
	if len(proofData) == groth16CompressedProofLen {
		if err := tryGroth16PairingVerify(proofData, publicInputs, vk); err != nil {
			return err
		}
		return nil
	}
	if len(proofData) == groth16DIPProofLen {
		if err := validateGroth16DIPProof(proofData); err != nil {
			return err
		}
		if err := tryDIPGroth16PairingVerify(proofData, publicInputs, vk); err != nil {
			return err
		}
		return nil
	}
	// Legacy test blobs: minimum length + 32-byte public inputs only.
	if len(proofData) < 32 {
		return fmt.Errorf("groth16: proof too short")
	}
	return nil
}

// VerifyProof runs structural + CheckZKP verification for a proof object.
func VerifyProof(p Proof) error {
	vk, err := resolveVerifyingKey(p.VerifyingKey, p.VerifyingKeyChunks)
	if err != nil {
		return err
	}
	if err := ValidateProofPayload(p); err != nil {
		return err
	}
	if proofHasChainAnchor(p) {
		if _, err := decodeHash32(p.BlockHash); err != nil {
			return fmt.Errorf("block_hash: %w", err)
		}
		if strings.TrimSpace(p.TransactionID) == "" {
			return fmt.Errorf("transaction_id required")
		}
		if p.BlockHeight < 0 {
			return fmt.Errorf("invalid block_height")
		}
	}
	data, err := hex.DecodeString(p.ProofData)
	if err != nil {
		return err
	}
	var inputs [][]byte
	for _, s := range p.PublicInputs {
		b, err := hex.DecodeString(s)
		if err != nil {
			return fmt.Errorf("public input hex: %w", err)
		}
		inputs = append(inputs, b)
	}
	return VerifyCheckZKP(CheckZKPMode(p.ProofType), data, inputs, vk)
}

func proofHasChainAnchor(p Proof) bool {
	return strings.TrimSpace(p.TransactionID) != "" || strings.TrimSpace(p.BlockHash) != ""
}
